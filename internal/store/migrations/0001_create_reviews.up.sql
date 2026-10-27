CREATE TABLE reviews (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    client_ip  text        NOT NULL,
    model      text        NOT NULL,
    "order"    jsonb       NOT NULL,
    result     jsonb       NOT NULL,
    cost_usd   numeric     NOT NULL DEFAULT 0 CHECK (cost_usd >= 0)
);

-- Recent reviews and the daily spend both read by time.
CREATE INDEX reviews_created_at_idx ON reviews (created_at DESC);
