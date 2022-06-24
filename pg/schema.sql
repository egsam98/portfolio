create table accounts (
    id bigserial primary key,
    name text not null unique,
    exchange_name text not null,
    key text not null,
    secret text not null,
    passphrase text,
    aliases text[]
);

create table portfolio_triggers (
    id uuid primary key,
    portfolio_id bigint not null,
    type text not null,
    currency text not null,
    created_at timestamp not null default now(),
    "limit" numeric,
    percent numeric,
    trailing_alert bool not null,
    start_total_cost numeric
);

-- name: Accounts_GetByName :one
select * from accounts where name = $1;

-- name: PortfolioTriggers_Create :copyfrom
insert into portfolio_triggers
    (id, portfolio_id, type, currency, "limit", percent, trailing_alert, start_total_cost, created_at) values
    ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: PortfolioTriggers_UpdateStartTotalCost :exec
update portfolio_triggers
set start_total_cost = $1 where id = $2 and type = 'COST_CHANGED_BY_PERCENT';

-- name: PortfolioTriggers_Delete :exec
delete from portfolio_triggers where id = $1;

-- name: PortfolioTriggers_DeleteByPortfolioID :exec
delete from portfolio_triggers where portfolio_id = $1;
