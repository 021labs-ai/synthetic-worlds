-- Synthetic Worlds schema
-- Standalone version: no FK to organizations or api_keys tables.

-- Imported traces (minimal: only stores what synthetic worlds needs)
CREATE TABLE IF NOT EXISTS traces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tool_spans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trace_id UUID NOT NULL REFERENCES traces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    input JSONB,
    output JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tool_spans_trace_id ON tool_spans(trace_id);
CREATE INDEX IF NOT EXISTS idx_tool_spans_name ON tool_spans(name);
CREATE INDEX IF NOT EXISTS idx_tool_spans_created_at ON tool_spans(created_at DESC);

-- Synthetic worlds (durable records)
CREATE TABLE IF NOT EXISTS synthetic_worlds (
    id UUID PRIMARY KEY,
    organization_id TEXT NOT NULL DEFAULT 'default',
    project_id TEXT NOT NULL DEFAULT 'default',
    api_key_id TEXT NOT NULL DEFAULT 'static',
    mode VARCHAR(20) NOT NULL,
    seed INTEGER,
    model VARCHAR(100),
    failure_profile JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    total_calls INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_synthetic_worlds_org_id ON synthetic_worlds(organization_id);
CREATE INDEX IF NOT EXISTS idx_synthetic_worlds_project_id ON synthetic_worlds(project_id);
CREATE INDEX IF NOT EXISTS idx_synthetic_worlds_status ON synthetic_worlds(status);

-- Synthetic calls (call history / analytics)
CREATE TABLE IF NOT EXISTS synthetic_calls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES synthetic_worlds(id) ON DELETE CASCADE,
    organization_id TEXT NOT NULL DEFAULT 'default',
    project_id TEXT NOT NULL DEFAULT 'default',
    tool_name TEXT NOT NULL,
    tool_schema TEXT,
    input_data TEXT,
    output_data TEXT,
    step_count INTEGER NOT NULL DEFAULT 0,
    mode_used VARCHAR(20) NOT NULL,
    cache_hit BOOLEAN NOT NULL DEFAULT FALSE,
    model_used VARCHAR(100),
    idempotency_key TEXT,
    duration_ms INTEGER,
    error TEXT,
    validation_data TEXT,
    feedback_data TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_synthetic_calls_world_id ON synthetic_calls(world_id);
CREATE INDEX IF NOT EXISTS idx_synthetic_calls_org_id ON synthetic_calls(organization_id);
CREATE INDEX IF NOT EXISTS idx_synthetic_calls_tool_name ON synthetic_calls(tool_name);
CREATE INDEX IF NOT EXISTS idx_synthetic_calls_created_at ON synthetic_calls(created_at DESC);
