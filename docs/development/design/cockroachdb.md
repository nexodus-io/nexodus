# Support Cockroach DB

> [Issue #598](https://github.com/nexodus-io/nexodus/issues/598)

## Summary

Support a DB technology that can reduce DB downtime of the service and allow the DB to horizontally scale.  

## Proposal

We should support Cockroach DB in addition to Postgres.  Cockroach shards and replicates over several work nodes allowing it to be horizontally scalable and have higher availability than traditional single node databases.  It also support doing zero downtime upgrades of the database.

Cockroach is already highly compatible with Postgres, it uses the same client drivers and supports most of the same SQL dialect.

### Known Issues and Limitations

* [GORM automigrations of unique fields breaks Cockroach DB #5752](https://github.com/go-gorm/gorm/issues/5752).  So you have to create the indexes manually.
* Cockroach DB's implements serializable optimistic transactions.  So if your [transactions have read/write contention](https://www.cockroachlabs.com/docs/v22.2/transactions#transaction-retries), some of the transactions will fail.  The apiserver should retry the transaction using the [`crdbgorm.ExecuteTx(...)` helper](https://www.cockroachlabs.com/docs/stable/build-a-go-app-with-cockroachdb-gorm.html).
* [There are statement input size limits:  16 Mib](https://www.cockroachlabs.com/docs/stable/known-limitations.html#size-limits-on-statement-input-from-sql-clients) : dont't think this should affect us.

## Alternatives Considered

* Don't support it - this would allow us to stay more nimble by only having to support one database type, but keeping the sevice up with a high level of 9's might be harder.

## References

* [How to choose between PostgreSQL and CockroachDB](https://docs.rackspace.com/blog/how-to-choose-between-postgresql-and-cockroachdb/)
* [Demo of CockroachDB's in-place rolling upgrades](https://dantheengineer.com/demo-of-cockroachdbs-in-place-rolling-upgrades/)
