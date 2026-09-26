with open("apps/api/main.go", "r") as f:
    c = f.read()

old_pgx = 'pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)'
new_pgx = '''config, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal(err)
	}
	config.ConnConfig.Tracer = otelpgx.NewTracer()
	pool, err := pgxpool.NewWithConfig(context.Background(), config)'''
c = c.replace(old_pgx, new_pgx)

if '"github.com/exaring/otelpgx"' not in c:
    c = c.replace('"github.com/jackc/pgx/v5/pgxpool"', '"github.com/jackc/pgx/v5/pgxpool"\n\t"github.com/exaring/otelpgx"')

old_tracer = 'cfg := config.Load()'
new_tracer = '''cfg := config.Load()
	tp, _ := router.InitTracer("api")
	if tp != nil {
		defer tp.Shutdown(context.Background())
	}'''
if 'tp, _ :=' not in c:
    c = c.replace(old_tracer, new_tracer)

with open("apps/api/main.go", "w") as f:
    f.write(c)
