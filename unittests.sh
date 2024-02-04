go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out

# need to run tests sequentially to avoid database issues. https://www.reddit.com/r/golang/comments/15n834m/pq_duplicate_key_value_violates_unique_constraint/
# TODO: inject some randomness in the database name to allow parallel tests
# for test in $(go list ./...); do go test "$test" -coverprofile=coverage.out && go tool cover -html=coverage.out ; done