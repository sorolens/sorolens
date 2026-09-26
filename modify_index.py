import os
if os.path.exists("apps/api/api/index.go"):
    with open("apps/api/api/index.go", "r") as f:
        c = f.read()

    old_pgx = 'pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))'
    new_pgx = '''config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
		if err == nil {
			config.ConnConfig.Tracer = otelpgx.NewTracer()
			pool, err = pgxpool.NewWithConfig(context.Background(), config)
		}'''
    c = c.replace(old_pgx, new_pgx)

    if '"github.com/exaring/otelpgx"' not in c:
        c = c.replace('"github.com/jackc/pgx/v5/pgxpool"', '"github.com/jackc/pgx/v5/pgxpool"\n\t"github.com/exaring/otelpgx"')

    with open("apps/api/api/index.go", "w") as f:
        f.write(c)
