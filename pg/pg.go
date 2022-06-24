package pg

import (
	"context"
	"net/url"
	"time"

	"github.com/egsam98/portfolio/pg/repo"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type DB struct {
	*pgxpool.Pool
	Queries repo.Querier
}

type Config struct {
	User            string
	Password        string
	Host            string
	Name            string
	DisableTLS      bool
	CertPath        string
	MaxOpenConns    int
	MaxConnLifeTime time.Duration
	Logger          zerolog.Logger
}

// URL returns database config in URL presentation
func (c *Config) URL() *url.URL {
	sslMode := "verify-full"
	if c.DisableTLS {
		sslMode = "disable"
	}
	q := make(url.Values)
	q.Set("sslmode", sslMode)
	if !c.DisableTLS {
		q.Set("sslrootcert", c.CertPath)
	}
	q.Set("timezone", "utc")

	return &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     c.Host,
		Path:     c.Name,
		RawQuery: q.Encode(),
	}
}

func NewDB(cfg *Config) (*DB, error) {
	dsn := cfg.URL().String()
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if cfg.MaxOpenConns < 1 {
		cfg.MaxOpenConns = 1
	}
	poolCfg.MinConns = int32(cfg.MaxOpenConns)
	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MaxConnLifetime = cfg.MaxConnLifeTime
	poolCfg.HealthCheckPeriod = time.Minute
	poolCfg.ConnConfig.Logger = newZerologAdapter(cfg.Logger)
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		conn.ConnInfo().RegisterDataType(pgtype.DataType{
			Value: &Decimal{},
			Name:  "numeric",
			OID:   pgtype.NumericOID,
		})
		return nil
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}
	return &DB{
		Pool:    pool,
		Queries: repo.New(pool),
	}, nil
}
