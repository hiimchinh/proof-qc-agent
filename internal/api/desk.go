package api

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	graphql "github.com/graph-gophers/graphql-go"

	"github.com/hiimchinh/proof-qc-agent/internal/agent"
	"github.com/hiimchinh/proof-qc-agent/internal/fleet"
	"github.com/hiimchinh/proof-qc-agent/internal/llm"
	"github.com/hiimchinh/proof-qc-agent/internal/store"
)

type deskView struct {
	ChallengerQueue         []string
	Simulated               bool
	Champion, Challenger    string
	SpentToday, DailyCapUsd float64
	Orders                  []deskOrderView
	Scoreboard              []fleetScoreView
	Events                  []fleetEventView
}
type deskOrderView struct {
	ID                                                              graphql.ID
	CaseID, Customer, Product, Comment, Expected, Status, CreatedAt string
	WidthIn, HeightIn                                               float64
	Runs                                                            []agentRunView
}
type agentRunView struct {
	Role, Model, Decision, DesignerNote, CustomerNote, Error, ErrorCause string
	Correct, NotesPass, OverriddenByRules, Scored                        bool
	CostUsd                                                              float64
	LatencyMs                                                            int32
	Trace                                                                []toolStepView
}
type toolStepView struct {
	Tool, Args, Result string
	Overridden         bool
}
type fleetScoreView struct {
	Model, Role                           string
	N                                     int32
	Accuracy, NotesPassRate, CostPerOrder float64
	P50Ms                                 int32
}
type fleetEventView struct {
	ID                                      graphql.ID
	Kind, Model, Reason, Numbers, CreatedAt string
}

func (r *resolver) Desk(ctx context.Context, args struct{ Limit int32 }) (*deskView, error) {
	if args.Limit < 1 || args.Limit > 50 {
		return nil, badInput("limit must be between 1 and 50")
	}
	orders, err := r.store.RecentOrders(ctx, int(args.Limit))
	if err != nil {
		return nil, r.deskError(ctx, err)
	}
	state, err := r.store.Fleet(ctx)
	if err != nil {
		return nil, r.deskError(ctx, err)
	}
	history, err := r.store.FleetModels(ctx)
	if err != nil {
		return nil, r.deskError(ctx, err)
	}
	runs, err := r.store.Runs(ctx)
	if err != nil {
		return nil, r.deskError(ctx, err)
	}
	events, err := r.store.FleetEvents(ctx, 30)
	if err != nil {
		return nil, r.deskError(ctx, err)
	}
	spent, err := r.SpentToday(ctx)
	if err != nil {
		return nil, err
	}
	out := &deskView{ChallengerQueue: fleet.RemainingQueue(state, history, r.challengerQueue), Simulated: true, Champion: state.Champion, Challenger: state.Challenger, SpentToday: spent, DailyCapUsd: r.capUSD,
		Orders: []deskOrderView{}, Scoreboard: []fleetScoreView{}, Events: []fleetEventView{}}
	for _, o := range orders {
		out.Orders = append(out.Orders, deskOrder(o))
	}
	for _, s := range fleet.Scoreboard(runs) {
		role := "retired"
		if s.Model == state.Champion {
			role = "champion"
		}
		if s.Model == state.Challenger {
			role = "challenger"
		}
		out.Scoreboard = append(out.Scoreboard, fleetScoreView{s.Model, role, int32(s.N), s.Accuracy, s.NotesPassRate, s.CostPerOrder, int32(s.P50 / time.Millisecond)})
	}
	for _, e := range events {
		out.Events = append(out.Events, fleetEventView{graphql.ID(strconv.FormatInt(e.ID, 10)), e.Kind, e.Model, e.Reason, string(e.Numbers), e.CreatedAt.UTC().Format(time.RFC3339)})
	}
	return out, nil
}

func (r *resolver) deskError(ctx context.Context, err error) error {
	r.log.ErrorContext(ctx, "read desk", "err", err)
	return errInternal
}

func deskOrder(o store.DeskOrder) deskOrderView {
	v := deskOrderView{ID: graphql.ID(strconv.FormatInt(o.ID, 10)), CaseID: o.CaseID, Customer: o.Order.Customer, Product: string(o.Order.Product),
		WidthIn: o.Order.WidthIn, HeightIn: o.Order.HeightIn, Comment: o.Order.Comment, Expected: string(o.Expected), Status: o.Status, CreatedAt: o.CreatedAt.UTC().Format(time.RFC3339), Runs: []agentRunView{}}
	for _, run := range o.Runs {
		rv := agentRunView{Role: run.Role, Model: run.Model, Decision: string(run.Result.Decision), DesignerNote: run.Result.DesignerNote, CustomerNote: run.Result.CustomerNote,
			Error: run.Error, ErrorCause: llm.ShortErrorCause(run.Error), Correct: run.Correct, NotesPass: run.NotesPass, Scored: !llm.IsProviderOutage(run.Error), OverriddenByRules: run.OverriddenByRules, CostUsd: run.CostUSD, LatencyMs: int32(run.Latency / time.Millisecond), Trace: []toolStepView{}}
		var steps []agent.TraceStep
		if json.Unmarshal(run.Trace, &steps) == nil {
			for _, step := range steps {
				rv.Trace = append(rv.Trace, toolStepView{step.Tool, string(step.Args), step.Result, step.Overridden})
			}
		}
		v.Runs = append(v.Runs, rv)
	}
	return v
}
