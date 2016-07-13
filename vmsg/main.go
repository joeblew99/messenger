// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/kr/text"
	"github.com/nlacasse/gocui"

	"v.io/v23"
	"v.io/v23/context"

	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"

	"messenger/internal"
)

const defaultEncryptionKey = "This is not secure!!!"

var (
	cmdRoot = &cmdline.Command{
		Name:  "vmsg",
		Short: "Runs the vanadium messenger service",
		Long:  "Runs the vanadium messenger service.",
		Children: []*cmdline.Command{
			cmdNode, cmdChat, cmdRobot,
		},
	}
	cmdNode = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runNode),
		Name:   "node",
		Short:  "Run the standalone node",
		Long:   "Run the standalone node.",
	}
	cmdChat = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runChat),
		Name:   "chat",
		Short:  "Run chat demo application",
		Long:   "Run chat demo application.",
	}
	cmdRobot = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runRobot),
		Name:   "robot",
		Short:  "Run a robot node that sends random messages every few seconds",
		Long:   "Run a robot node that sends random messages every few seconds.",
	}

	advertisementId   string
	enableLocalDisc   bool
	globalDiscPaths   string
	storeDir          string
	maxActivePeers    int
	maxHops           int
	rateAclInJson     string
	rateAclOutJson    string
	rateAclSenderJson string
	encryptionKey     string

	incomingDir string
)

func main() {
	if f := flag.Lookup("alsologtostderr"); f != nil {
		f.Value.Set("false")
	}
	if f := flag.Lookup("stderrthreshold"); f != nil {
		f.Value.Set("FATAL")
	}
	cmdRoot.Flags.StringVar(&advertisementId, "advertisement-id", "", "The advertisement ID to use. If left empty, a random one is generated.")
	cmdRoot.Flags.BoolVar(&enableLocalDisc, "enable-local-discovery", true, "Whether local discovery, i.e. using mDNS and/or BLE, should be enabled.")
	cmdRoot.Flags.StringVar(&globalDiscPaths, "global-discovery-paths", "", "A comma-separated list of namespace paths to use for global discovery.")
	cmdRoot.Flags.StringVar(&storeDir, "store-dir", "", "The name of the local directory where to store the messages.")
	cmdRoot.Flags.IntVar(&maxActivePeers, "max-active-peers", 2, "The maximum number of peers to send updates to concurrently.")
	cmdRoot.Flags.IntVar(&maxHops, "max-hops", 50, "The maximum number of hops that a message can go through.")
	cmdRoot.Flags.StringVar(&rateAclInJson, "rate-acl-in", `[{"acl":{"In":["..."]},"limit":20}]`, "The RateAcl to authorize incoming RPCs, in JSON format")
	cmdRoot.Flags.StringVar(&rateAclOutJson, "rate-acl-out", `[{"acl":{"In":["..."]},"limit":100}]`, "The RateAcl to authorize outgoing RPCs, in JSON format")
	cmdRoot.Flags.StringVar(&rateAclSenderJson, "rate-acl-sender", `[{"acl":{"In":["..."]},"limit":100}]`, "The RateAcl to authorize the sender of incoming messages, in JSON format")
	cmdRoot.Flags.StringVar(&encryptionKey, "encryption-key", defaultEncryptionKey, "Messages are encrypted with AES256 using this key")

	cmdChat.Flags.StringVar(&incomingDir, "incoming-dir", os.TempDir(), "The directory where to save incoming files")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

func paramsFromFlags(ctx *context.T, env *cmdline.Env) (params internal.Params, err error) {
	if storeDir == "" {
		err = env.UsageErrorf("--store-dir must be set")
		return
	}
	var paths []string
	if globalDiscPaths != "" {
		paths = strings.Split(globalDiscPaths, ",")
	}

	params.AdvertisementID = advertisementId
	params.EnableLocalDiscovery = enableLocalDisc
	params.GlobalDiscoveryPaths = paths
	params.MaxActivePeers = maxActivePeers
	params.MaxHops = maxHops
	params.MountTTL = 20 * time.Second
	params.ScanInterval = 10 * time.Second
	params.Store = internal.NewFileStorage(ctx, storeDir)

	if params.RateAclIn, err = internal.RateAclFromJSON([]byte(rateAclInJson)); err != nil {
		return
	}
	if params.RateAclOut, err = internal.RateAclFromJSON([]byte(rateAclOutJson)); err != nil {
		return
	}
	if params.RateAclSender, err = internal.RateAclFromJSON([]byte(rateAclSenderJson)); err != nil {
		return
	}
	return
}

func runNode(ctx *context.T, env *cmdline.Env, args []string) error {
	params, err := paramsFromFlags(ctx, env)
	if err != nil {
		return err
	}
	server, _, stop, err := internal.StartNode(ctx, params)
	if err != nil {
		return err
	}
	defer stop()
	ctx.Infof("Listening on: %v", server.Status().Endpoints)
	<-signals.ShutdownOnSignals(ctx)
	return nil
}

func runChat(ctx *context.T, env *cmdline.Env, args []string) error {
	params, err := paramsFromFlags(ctx, env)
	if err != nil {
		return err
	}

	_, ps, stop, err := internal.StartNode(ctx, params)
	if err != nil {
		return err
	}
	defer stop()

	g := gocui.NewGui()
	if err := g.Init(); err != nil {
		return err
	}
	defer g.Close()
	g.ShowCursor = true
	g.SetLayout(func(g *gocui.Gui) error {
		maxX, maxY := g.Size()
		messageInputViewHeight := 3
		if _, err := g.SetView("history", -1, -1, maxX, maxY-messageInputViewHeight); err != nil {
			if err != gocui.ErrorUnkView {
				return err
			}
		}
		if messageInputView, err := g.SetView("messageInput", -1, maxY-messageInputViewHeight, maxX, maxY-1); err != nil {
			if err != gocui.ErrorUnkView {
				return err
			}
			messageInputView.Editable = true
		}
		if err := g.SetCurrentView("messageInput"); err != nil {
			return err
		}
		return nil
	})
	g.Flush()

	historyView, err := g.View("history")
	if err != nil {
		return err
	}

	help := func() {
		historyView.Write([]byte("*** Welcome to Vanadium Peer to Peer Chat ***"))
		historyView.Write([]byte("***"))
		historyView.Write([]byte(color.RedString("*** This is a demo application.")))
		historyView.Write([]byte("***"))
		if encryptionKey == defaultEncryptionKey {
			historyView.Write([]byte(color.RedString("*** Messages are encrypted with the default key. They are NOT private.")))
		}
		historyView.Write([]byte("***"))
		historyView.Write([]byte("*** Messages are stored and relayed peer-to-peer for 15 minutes after they are"))
		historyView.Write([]byte("*** created. New peers will see up to 15 minutes of history when they join."))
		historyView.Write([]byte("***"))
		historyView.Write([]byte("*** Available commands are:"))
		historyView.Write([]byte("***   /help to see this help message"))
		historyView.Write([]byte("***   /ping to send a ping"))
		historyView.Write([]byte("***   /send <filename> to send a file"))
		historyView.Write([]byte("***"))
		historyView.Write([]byte("*** Press Ctrl-C to exit."))
		historyView.Write([]byte("***"))
		g.Flush()
	}

	if err := g.SetKeybinding("", gocui.KeyCtrlC, 0,
		func(g *gocui.Gui, v *gocui.View) error {
			return gocui.Quit
		},
	); err != nil {
		return err
	}
	if err := g.SetKeybinding("messageInput", gocui.KeyEnter, 0,
		func(g *gocui.Gui, v *gocui.View) error {
			defer g.Flush()
			mtxt := strings.TrimSpace(v.Buffer())
			v.Clear()
			if mtxt == "" {
				return nil
			}
			fname := ""
			switch {
			case mtxt == "/clear":
				historyView.Clear()
				historyView.SetOrigin(0, 0)
				return nil
			case mtxt == "/help":
				help()
				return nil
			case mtxt == "/ping":
				mtxt = fmt.Sprintf("\x01PING %d", time.Now().UnixNano())
			case strings.HasPrefix(mtxt, "/send"):
				fname = strings.TrimSpace(mtxt[5:])
				mtxt = ""
			case strings.HasPrefix(mtxt, "/"):
				fmt.Fprintf(historyView, "### Unknown command %s", mtxt)
				return nil
			}
			if err := sendMessage(ctx, ps, params.Store, mtxt, fname); err != nil {
				fmt.Fprintf(historyView, "## sendMessage failed: %v\n", err)
			}
			return nil
		},
	); err != nil {
		return err
	}

	help()

	go func() {
		for msg := range ps.Sub() {
			_, r, err := params.Store.OpenRead(ctx, msg.Id)
			if err != nil {
				continue
			}
			msgText, filename, err := decryptChatMessage(msg.Id, r, incomingDir)
			r.Close()
			if err != nil {
				fmt.Fprintf(historyView, "## decryptChatMessage failed: %v\n", err)
				g.Flush()
				continue
			}

			delta := time.Since(msg.CreationTime).Seconds()
			hops := len(msg.Hops)
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "%s %2d %5.2fs ", msg.CreationTime.Local().Format("15:04:05"), hops, delta)
			if msgText != "" {
				if strings.HasPrefix(msgText, "\x01PING") {
					fmt.Fprintf(&buf, "PING from %s", msg.SenderBlessings)
					reply := "\x01PONG" + msgText[5:]
					if err := sendMessage(ctx, ps, params.Store, reply, ""); err != nil {
						ctx.Errorf("sendMessage failed: %v", err)
					}
				} else if strings.HasPrefix(msgText, "\x01PONG ") {
					if i, err := strconv.ParseInt(msgText[6:], 10, 64); err == nil {
						t := time.Unix(0, i)
						fmt.Fprintf(&buf, "PING reply from %s: %s", msg.SenderBlessings, time.Since(t))
					}
				} else {
					fmt.Fprintf(&buf, "<%s> %s", msg.SenderBlessings, msgText)
				}
			}
			if filename != "" {
				fmt.Fprintf(&buf, "Received file from %s: %s", msg.SenderBlessings, filename)
			}

			width, height := historyView.Size()
			historyView.Write(text.WrapBytes(buf.Bytes(), width))
			numLines := historyView.NumberOfLines()
			if numLines > height {
				historyView.SetOrigin(0, numLines-height)
			}
			g.Flush()
		}
	}()

	if err := g.MainLoop(); err != nil && err != gocui.Quit {
		return err
	}
	return nil
}

func runRobot(ctx *context.T, env *cmdline.Env, args []string) error {
	params, err := paramsFromFlags(ctx, env)
	if err != nil {
		return err
	}
	_, ps, stop, err := internal.StartNode(ctx, params)
	if err != nil {
		return err
	}
	defer stop()

	go func() {
		for {
			// Send a message every 15 to 45 seconds.
			n := rand.Intn(30) + 15
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(n) * time.Second):
				mtxt := fmt.Sprintf("Your lucky number is %d", n)
				if err := sendMessage(ctx, ps, params.Store, mtxt, ""); err != nil {
					ctx.Errorf("sendMessage failed: %v", err)
				}
			}
		}
	}()

	go func() {
		for msg := range ps.Sub() {
			_, r, err := params.Store.OpenRead(ctx, msg.Id)
			if err != nil {
				continue
			}
			msgText, filename, err := decryptChatMessage(msg.Id, r, incomingDir)
			r.Close()
			if err != nil {
				ctx.Infof("decryptChatMessage failed: %v", err)
				continue
			}
			ctx.Infof("Incoming message from %s %q %q", msg.SenderBlessings, msgText, filename)
			if strings.HasPrefix(msgText, "\x01PING") {
				reply := "\x01PONG" + msgText[5:]
				if err := sendMessage(ctx, ps, params.Store, reply, ""); err != nil {
					ctx.Errorf("sendMessage failed: %v", err)
				}
			}
		}
	}()

	<-signals.ShutdownOnSignals(ctx)
	return nil
}

func sendMessage(ctx *context.T, ps *internal.PubSub, store internal.MessengerStorage, txt, fname string) error {
	msgId := internal.NewMessageId()
	encryptedFile, err := encryptChatMessage(msgId, txt, fname)
	if err != nil {
		return err
	}
	defer os.Remove(encryptedFile)

	p := v23.GetPrincipal(ctx)
	msg, err := internal.NewMessageFromFile(encryptedFile)
	if err != nil {
		return err
	}
	msg.Id = msgId
	msg.SenderBlessings, _ = p.BlessingStore().Default()
	msg.Lifespan = 15 * time.Minute
	msg.Signature, err = p.Sign(msg.Hash())
	if err != nil {
		return err
	}
	w, err := store.OpenWrite(ctx, msg, 0)
	if err != nil {
		return err
	}
	in, err := os.Open(encryptedFile)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	ctx.Infof("New message id %s stored", msg.Id)
	ps.Pub(msg)
	return nil
}

func encryptChatMessage(id, text, attachment string) (string, error) {
	tmpfile, err := ioutil.TempFile("", "vmsg-encrypt-")
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()
	enc, err := aesEncoder(encryptionKey, tmpfile)
	if err != nil {
		return "", err
	}
	w := multipart.NewWriter(enc)
	if err := w.SetBoundary(id); err != nil {
		return "", err
	}

	// Write text field.
	if err := w.WriteField("text", text); err != nil {
		return "", err
	}
	// Write attachment, if provided.
	if attachment != "" {
		aw, err := w.CreateFormFile("attachment", filepath.Base(attachment))
		if err != nil {
			return "", err
		}
		ar, err := os.Open(attachment)
		if err != nil {
			return "", err
		}
		defer ar.Close()
		if _, err := io.Copy(aw, ar); err != nil {
			return "", err
		}
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

func decryptChatMessage(id string, msgReader io.Reader, dir string) (text, filename string, err error) {
	dec, err := aesDecoder(encryptionKey, msgReader)
	if err != nil {
		return "", "", err
	}
	r := multipart.NewReader(dec, id)
	form, err := r.ReadForm(1 << 20)
	if err != nil {
		return "", "", err
	}
	defer form.RemoveAll()
	if t := form.Value["text"]; len(t) == 1 {
		text = t[0]
	}
	if a := form.File["attachment"]; len(a) == 1 {
		fh := a[0]
		in, err := fh.Open()
		if err != nil {
			return "", "", err
		}
		defer in.Close()
		filename = filepath.Join(dir, filepath.Base(fh.Filename))
		out, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return "", "", err
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			return "", "", err
		}
	}
	return text, filename, nil
}
