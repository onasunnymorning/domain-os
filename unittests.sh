# start db server
docker volume rm domain-os_db
docker run --rm -e POSTGRES_PASSWORD=unittest -e POSTGRES_USER=postgres --name testdb -d -p 5432:5432 postgres:16.1
# run tests
go test ./... -coverpkg=./... -coverprofile=coverage.out && go tool cover -html=coverage.out
# stop db server
docker stop testdb

# need to run tests sequentially to avoid database issues. https://www.reddit.com/r/golang/comments/15n834m/pq_duplicate_key_value_violates_unique_constraint/
# TODO: inject some randomness in the database name to allow parallel tests
# for test in $(go list ./...); do go test "$test" -coverprofile=coverage.out && go tool cover -html=coverage.out ; done