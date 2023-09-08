export CGO_ENABLED=0
export GOOS=linux
go mod download
go mod tidy
go build -x -o ./build/gopull
