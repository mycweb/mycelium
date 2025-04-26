package mychui

import (
	"bytes"
	"context"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	"go.brendoncarroll.net/exp/slices2"
	"go.brendoncarroll.net/stdctx/logctx"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/myccanon/mychtml"
	"myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss"
)

func Serve(ctx context.Context, l net.Listener, sys *mycss.System) error {
	return New(sys).Serve(ctx, l)
}

// devPath is the path to the views from the directory the application is run.
// when it is empty the embeded views are used.
var devPath = "" // "./mycss/mychui"

type Server struct {
	sys   *mycss.System
	app   *fiber.App
	bgCtx context.Context
}

func New(sys *mycss.System) *Server {
	s := &Server{sys: sys}

	var renderer *html.Engine
	if devPath != "" {
		renderer = html.New(devPath, ".html")
		renderer.Reload(true)
	} else {
		renderer = html.NewFileSystem(http.FS(viewFS), ".html")
	}
	renderer.AddFunc("hexDump", func(x []byte) string {
		return hex.Dump(x)
	})
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		Views:                 renderer,
	})
	// views
	app.Get("/", s.home)
	app.Post("/pod", s.postPod)
	app.Get("/pod/:podID", s.pod)
	app.Post("/pod/:podID/drop", s.dropPod)

	v1 := app.Group("/v1")
	v1.Get("/pod/:podID/renderHTML", s.renderHTML)
	v1.Get("/pod/:podID/blob/:ref", s.blob)
	v1.Get("/pod/:podID/ws", websocket.New(s.handleWS))
	s.app = app
	return s
}

func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	s.bgCtx = ctx
	logctx.Infof(ctx, "serving on %v", l.Addr())
	return s.app.Listener(l)
}

func (s *Server) home(c *fiber.Ctx) error {
	ctx := c.Context()
	pods, err := s.sys.List(ctx)
	if err != nil {
		return err
	}
	slices.SortFunc(pods, func(a, b *mycss.Pod) int {
		return int(a.ID() - b.ID())
	})
	podInfos := slices2.Map(pods, func(p *mycss.Pod) podInfo {
		info, err := makePodInfo(ctx, p)
		if err != nil {
			logctx.Error(ctx, "pod infos", zap.Error(err))
		}
		return *info
	})
	return c.Render("view/home", struct {
		Hostname string
		Pods     []podInfo
	}{
		Hostname: c.Hostname(),
		Pods:     podInfos,
	}, "view/layout")
}

func (s *Server) postPod(c *fiber.Ctx) error {
	ctx := c.Context()
	pod, err := s.sys.Create(ctx)
	if err != nil {
		return err
	}
	if devPath != "" {
		for i := 0; i < rand.Intn(20); i++ {
			if err := pod.Put(ctx, nil, fmt.Sprintf("key-%d", i), mycmem.NewB32(i)); err != nil {
				return err
			}
		}
		if err := SetRenderHTML(ctx, pod, mychtml.HTML{Type: "b", Text: "hello world"}); err != nil {
			return err
		}
	}
	return c.RedirectBack("/")
}

func (s *Server) pod(c *fiber.Ctx) error {
	pod, err := s.getPod(c)
	if err != nil {
		return err
	}
	info, err := makePodInfo(c.Context(), pod)
	if err != nil {
		return err
	}
	return c.Render("view/pod", struct {
		Hostname string
		Pod      podInfo
	}{
		Hostname: c.Hostname(),
		Pod:      *info,
	}, "view/layout")
}

func (s *Server) dropPod(c *fiber.Ctx) error {
	ctx := c.Context()
	pod, err := s.getPod(c)
	if err != nil {
		return err
	}
	if err := s.sys.Drop(ctx, pod.ID()); err != nil {
		return err
	}
	return c.Redirect("/")
}

func (s *Server) renderHTML(c *fiber.Ctx) error {
	ctx := c.Context()
	pod, err := s.getPod(c)
	if err != nil {
		return err
	}
	store := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
	htmlVal, err := mycss.Eval(ctx, pod, store, store, RenderLazy)
	if err != nil {
		return fmt.Errorf("eval renderHTML: %w", err)
	}
	htmlNode, err := mychtml.DecodeHTML(ctx, store, htmlVal)
	if err != nil {
		return fmt.Errorf("decoding html: %w", err)
	}
	c = c.Type("html")
	return renderHTMLDoc(c, htmlNode)
}

var base64Codec = mycmem.Base64Encoding()

func (s *Server) blob(c *fiber.Ctx) error {
	ctx := c.Context()
	pod, err := s.getPod(c)
	if err != nil {
		return err
	}
	refData, err := base64Codec.DecodeString(c.Params("ref"))
	if err != nil {
		return err
	}
	if len(refData) != mycmem.RefBytes {
		return fmt.Errorf("ref must decoded to 32 bytes. HAVE len=%d", len(refData))
	}
	var cid cadata.ID
	copy(cid[:], refData)
	var buf [mycelium.MaxSizeBytes]byte
	n, err := pod.Store().Get(ctx, &cid, nil, buf[:])
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "application/bytes")
	_, err = c.Write(buf[:n])
	return err
}

func (s *Server) getPod(c *fiber.Ctx) (*mycss.Pod, error) {
	ctx := c.Context()
	podID, err := strconv.Atoi(c.Params("podID"))
	if err != nil {
		return nil, err
	}
	return s.sys.Get(ctx, mycss.PodID(podID))
}

func (s *Server) handleWS(c *websocket.Conn) {
	ctx := s.bgCtx
	podID, err := strconv.Atoi(c.Params("podID"))
	if err != nil {
		return
	}
	logctx.Info(ctx, "started websocket", zap.Int("pod", podID))
	defer logctx.Info(ctx, "closing websocket", zap.Int("pod", podID))

	if err := func() error {
		ctx, cf := context.WithCancel(ctx)
		defer cf()
		pod, err := s.sys.Get(ctx, mycss.PodID(podID))
		if err != nil {
			return err
		}
		return pod.DoInProcess(ctx, func(pc mycss.ProcCtx) error {
			laz := RenderLazy(pc.NS().ToMycelium())
			notif := make(chan mycss.Notif)
			pc.Subscribe(notif)
			defer pc.Unsubscribe(notif)

			buf := bytes.Buffer{}
			for i := 0; ; i++ {
				buf.Reset()
				out, err := pc.Eval(ctx, laz)
				if err != nil {
					return err
				}
				node, err := mychtml.DecodeHTML(ctx, pc.Store(), out)
				if err != nil {
					return err
				}
				if err := renderHTMLNode(&buf, node); err != nil {
					return err
				}
				if err := c.WriteJSON(map[string]any{
					"prev": "",
					"next": "",
					"data": buf.String(),
				}); err != nil {
					return err
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-notif:
				}
			}
		})
	}(); err != nil {
		logctx.Error(ctx, "handling websocket", zap.Error(err))
		return
	}
}

//go:embed view/*
var viewFS embed.FS

type podInfo struct {
	ID        mycss.PodID
	NS        []nsEntry
	NSCount   int64
	ProcCount int64
	Config    string
	CreatedAt time.Time
}

func makePodInfo(ctx context.Context, pod *mycss.Pod) (*podInfo, error) {
	ns, err := pod.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	cfg := pod.Config()
	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return &podInfo{
		ID:        pod.ID(),
		ProcCount: pod.ProcCount(),
		NSCount:   int64(len(ns)),
		NS:        makeNSEntries(ns),
		CreatedAt: pod.CreatedAt(),
		Config:    string(cfgData),
	}, nil
}

type nsEntry struct {
	Key   string
	Value value
}

func makeNSEntries(x myccanon.Namespace) (ret []nsEntry) {
	keys := maps.Keys(x)
	slices.Sort(keys)
	for _, k := range keys {
		v := x[k]
		ret = append(ret, nsEntry{
			Key:   k,
			Value: newValue(v),
		})
	}
	return ret
}

type value struct {
	Type   string
	Pretty string
	Raw    []byte
	Bits   int
}

func newValue(x mycmem.Value) value {
	data := mycmem.MarshalAppend(nil, x)
	return value{
		Type:   mycmem.Pretty(x.Type()),
		Pretty: mycmem.Pretty(x),
		Raw:    data,
		Bits:   x.Type().SizeOf(),
	}
}

func (v value) String() string {
	return v.Pretty
}
