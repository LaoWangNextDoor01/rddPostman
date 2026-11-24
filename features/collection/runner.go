package collection

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/transport"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

type runnerItemResult struct {
	NodeID      domain.ID
	Name        string
	Method      string
	URL         string
	StatusCode  int
	Duration    time.Duration
	Size        int64
	Error       string
	TestResults []domain.TestResult
}

type collectionRunnerState struct {
	mu        sync.Mutex
	targetID  string
	running   bool
	done      bool
	total     int
	completed int
	startedAt time.Time
	elapsed   time.Duration
	results   []runnerItemResult
	cancel    context.CancelFunc
}

type runnerSnapshot struct {
	running   bool
	done      bool
	total     int
	completed int
	elapsed   time.Duration
	results   []runnerItemResult
}

func (r *collectionRunnerState) snapshot() runnerSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]runnerItemResult, len(r.results))
	copy(out, r.results)
	return runnerSnapshot{
		running:   r.running,
		done:      r.done,
		total:     r.total,
		completed: r.completed,
		elapsed:   r.elapsed,
		results:   out,
	}
}

func (r *collectionRunnerState) stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func collectRunRequests(items []*domain.Node) []*domain.Node {
	var out []*domain.Node
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.Kind == domain.KindRequest && item.Request != nil {
			out = append(out, item)
		} else if len(item.Children) > 0 {
			out = append(out, collectRunRequests(item.Children)...)
		}
	}
	return out
}

func renderCollectionRunner(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node, id string) {
	tk := thV2.Of(c)
	m := thV2.Metrics

	var items []*domain.Node
	folderTot := 0
	if node != nil && node.Kind == domain.KindFolder {
		items = node.Children
		folderTot = folderTotal(node.Children)
	} else if coll != nil {
		items = coll.Items
		folderTot = folderTotal(coll.Items)
	}
	requests := collectRunRequests(items)
	reqTotal := len(requests)

	host := ui.Scroll(c.Key("coll-runner-" + id)).Grow(1)
	runner := *ui.Local(host, "runner-state", func() *collectionRunnerState {
		return &collectionRunnerState{targetID: id}
	})
	if runner.targetID != id {
		runner.stop()
		runner = &collectionRunnerState{targetID: id}
	}
	snap := runner.snapshot()

	startRun := func() {
		if reqTotal == 0 || coll == nil || props.actions.runRequestSync == nil {
			return
		}
		runner.stop()
		ctx, cancel := context.WithCancel(context.Background())
		reqList := make([]*domain.Node, len(requests))
		copy(reqList, requests)
		collID := coll.ID
		runFn := props.actions.runRequestSync
		services := c.Services()

		runner.mu.Lock()
		runner.running = true
		runner.done = false
		runner.total = len(reqList)
		runner.completed = 0
		runner.startedAt = time.Now()
		runner.elapsed = 0
		runner.results = nil
		runner.cancel = cancel
		runner.mu.Unlock()

		go func() {
			defer cancel()
			start := time.Now()
			for _, reqNode := range reqList {
				if ctx.Err() != nil {
					break
				}
				resp := runFn(ctx, collID, reqNode.ID, *reqNode.Request.Clone(), reqNode.Auth)
				itemRes := runnerItemResult{
					NodeID: reqNode.ID,
					Name:   reqNode.Name,
					Method: reqNode.Request.Method,
					URL:    reqNode.Request.URL,
				}
				if resp != nil {
					itemRes.StatusCode = resp.StatusCode
					itemRes.Duration = resp.Duration
					itemRes.Size = resp.Size
					itemRes.Error = resp.Error
					itemRes.TestResults = resp.TestResults
				}
				runner.mu.Lock()
				runner.results = append(runner.results, itemRes)
				runner.completed++
				runner.elapsed = time.Since(start)
				runner.mu.Unlock()
				services.Invalidate()
			}
			runner.mu.Lock()
			runner.running = false
			runner.done = true
			runner.elapsed = time.Since(start)
			runner.cancel = nil
			runner.mu.Unlock()
			services.Invalidate()
		}()
	}

	host.Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(docWidth).Padding(32, 0, 40, 0).Gap(20).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, locales.RunnerTitle()).FontSize(18).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, locales.RunnerDesc()).
						FontSize(13).TextColor(tk.TextMuted)
				})

				ui.Box(c).FillWidth().Height(1).Background(tk.Border)

				// 概览统计卡片
				passedCount, failedCount := 0, 0
				for _, res := range snap.results {
					if res.Error != "" || res.StatusCode == 0 || res.StatusCode >= 400 {
						failedCount++
						continue
					}
					allTestsOK := true
					for _, tr := range res.TestResults {
						if !tr.Passed {
							allTestsOK = false
							break
						}
					}
					if allTestsOK {
						passedCount++
					} else {
						failedCount++
					}
				}

				ui.Row(c).Gap(12).Children(func() {
					runnerStatCard(c, locales.RunnerStatRequests(), fmt.Sprintf("%d", reqTotal), tk.TextPrimary)
					runnerStatCard(c, locales.RunnerStatFolders(), fmt.Sprintf("%d", folderTot), tk.TextPrimary)
					if snap.running || snap.done {
						runnerStatCard(c, locales.RunnerStatPassed(), fmt.Sprintf("%d", passedCount), tk.Success)
						runnerStatCard(c, locales.RunnerStatFailed(), fmt.Sprintf("%d", failedCount), tk.Danger)
						runnerStatCard(c, locales.RunnerStatDuration(), transport.FormatDuration(snap.elapsed), tk.TextSecondary)
					}
				})

				// 操作按钮行
				ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
					if snap.running {
						stopBtn := ui.ButtonBase(c.Key("coll-run-stop")).
							Height(34).Padding(0, 20).Radius(6).
							Background(tk.DangerSoft).Border(1, tk.Danger).
							AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).
							Role(ui.RoleButton).Label(locales.RunnerStopLabel())
						if stopBtn.Clicked() {
							runner.stop()
						}
						stopBtn.Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons.X).FontSize(13).TextColor(tk.Danger)
								ui.Text(c, locales.RunnerStopProgress(strconv.Itoa(snap.completed), strconv.Itoa(snap.total))).
									FontSize(13).FontWeight(500).TextColor(tk.Danger)
							})
						})
					} else {
						runBtn := ui.ButtonBase(c.Key("coll-run-start")).
							Height(34).Padding(0, 20).Radius(6).
							AlignItems(ui.Center).Justify(ui.Center).
							Role(ui.RoleButton).Label(locales.RunnerRunBtn()).
							Disabled(reqTotal == 0).
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						if reqTotal == 0 {
							runBtn.Background(tk.BgSubtle)
						} else {
							runBtn.Cursor(ui.CursorPointer)
							switch {
							case runBtn.Pressed():
								runBtn.Background(tk.AccentPressed)
							case runBtn.Hovered():
								runBtn.Background(tk.AccentHover)
							default:
								runBtn.Background(tk.Accent)
							}
							if runBtn.Clicked() {
								startRun()
							}
						}
						runBtn.Children(func() {
							fg := tk.OnAccent
							if reqTotal == 0 {
								fg = tk.TextMuted
							}
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons.Play).FontSize(13).TextColor(fg)
								ui.Text(c, locales.RunnerRunBtn()).FontSize(13).FontWeight(500).TextColor(fg)
							})
						})
					}

					if reqTotal == 0 {
						ui.Text(c, locales.RunnerEmptyHint()).FontSize(m.FontXS).TextColor(tk.TextMuted)
					} else if snap.running {
						ui.Text(c, locales.RunnerRunningHint(strconv.Itoa(snap.completed+1), strconv.Itoa(snap.total))).
							FontSize(m.FontXS).TextColor(tk.TextSecondary)
					} else if snap.done {
						ui.Text(c, locales.RunnerDoneHint(len(snap.results))).
							FontSize(m.FontXS).TextColor(tk.TextSecondary)
					}
				})

				// 执行结果列表
				if len(snap.results) > 0 {
					ui.Column(c).FillWidth().Radius(6).Border(1, tk.Border).Background(tk.BgSurface).Clip().Children(func() {
						for idx, res := range snap.results {
							r := res
							row := ui.ButtonBase(c.Key(fmt.Sprintf("runner-res-%d-%s", idx, r.NodeID))).
								FillWidth().Padding(10, 14).Gap(10).
								AlignItems(ui.Center).Justify(ui.Start).
								Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(r.Name)
							if idx > 0 {
								row.BorderWidth(1, 0, 0, 0).BorderColor(tk.Border)
							}
							if row.Hovered() {
								row.Background(tk.BgHover)
							}
							if row.Clicked() && props.actions.openTab != nil {
								props.actions.openTab(string(r.NodeID))
							}
							row.Children(func() {
								ui.Text(c, r.Method).FontSize(11).Bold().
									TextColor(methodColorV2(tk, r.Method)).Width(48).SingleLine()

								ui.Column(c).Grow(1).Gap(2).Children(func() {
									ui.Text(c, r.Name).FontSize(13).FontWeight(500).
										TextColor(tk.TextPrimary).SingleLine()
									if r.Error != "" {
										ui.Text(c, r.Error).FontSize(11).TextColor(tk.Danger).SingleLine()
									} else {
										ui.Text(c, r.URL).FontSize(11).TextColor(tk.TextMuted).SingleLine()
									}
									if len(r.TestResults) > 0 {
										passed := 0
										for _, tr := range r.TestResults {
											if tr.Passed {
												passed++
											}
										}
										clr := tk.Success
										if passed < len(r.TestResults) {
											clr = tk.Danger
										}
										ui.Text(c, locales.RunnerTestsRatio(strconv.Itoa(passed), strconv.Itoa(len(r.TestResults)))).
											FontSize(11).TextColor(clr).SingleLine()
									}
								})

								statusColor := tk.Success
								statusText := fmt.Sprintf("%d", r.StatusCode)
								if r.Error != "" || r.StatusCode == 0 {
									statusColor = tk.Danger
									statusText = "ERR"
								} else if r.StatusCode >= 400 {
									statusColor = tk.Danger
								} else if r.StatusCode >= 300 {
									statusColor = tk.Info
								}
								ui.Text(c, statusText).FontSize(12).Bold().TextColor(statusColor).SingleLine()
								ui.Text(c, transport.FormatDuration(r.Duration)).FontSize(11).TextColor(tk.TextMuted).SingleLine()
							})
						}
					})
				}
			})
		})
	})
}

func runnerStatCard(c *ui.Context, label, value string, valColor ui.Color) {
	tk := thV2.Of(c)
	m := thV2.Metrics
	ui.Box(c).Padding(12, 16).Radius(m.RadiusSM).Background(tk.BgSubtle).
		Border(1, tk.Border).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, label).FontSize(11).TextColor(tk.TextMuted).SingleLine()
			ui.Text(c, value).FontSize(20).FontWeight(600).TextColor(valColor).SingleLine()
		})
	})
}

func methodColorV2(tk thV2.Tokens, method string) ui.Color {
	switch method {
	case "GET":
		return tk.MethodGet
	case "POST":
		return tk.MethodPost
	case "PUT":
		return tk.MethodPut
	case "DELETE":
		return tk.MethodDelete
	case "PATCH":
		return tk.MethodPatch
	default:
		return tk.TextMuted
	}
}
