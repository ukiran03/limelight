CREATE TABLE if NOT EXISTS movies (
   id bigserial PRIMARY KEY,
   created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
   title text NOT NULL,
   year INTEGER NOT NULL,
   runtime INTEGER NOT NULL,
   genres text[] NOT NULL,
   version INTEGER NOT NULL DEFAULT 1
)
