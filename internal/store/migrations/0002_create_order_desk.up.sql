CREATE TABLE desk_orders (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id text NOT NULL,
    "order" jsonb NOT NULL,
    expected text NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','working','done','failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX desk_orders_queue_idx ON desk_orders (created_at,id) WHERE status='queued';
CREATE TABLE agent_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id bigint NOT NULL REFERENCES desk_orders(id),
    role text NOT NULL CHECK(role IN ('champion','challenger')),
    model text NOT NULL,
    result jsonb NOT NULL,
    expected text NOT NULL,
    correct boolean NOT NULL,
    notes_pass boolean NOT NULL,
    cost_usd numeric NOT NULL CHECK(cost_usd>=0),
    latency_ns bigint NOT NULL CHECK(latency_ns>=0),
    trace jsonb NOT NULL DEFAULT '[]',
    overridden_by_rules boolean NOT NULL DEFAULT false,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(order_id,role)
);
CREATE INDEX agent_runs_created_at_idx ON agent_runs(created_at);
CREATE TABLE fleet_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
    champion text NOT NULL,
    challenger text NOT NULL
);
CREATE TABLE fleet_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind text NOT NULL,
    model text NOT NULL,
    reason text NOT NULL,
    numbers jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE SEQUENCE desk_case_cursor MINVALUE 0 START 0;
