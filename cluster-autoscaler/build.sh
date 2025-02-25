export GOOS=linux
export GOARCH="amd64"
go build -o cluster-autoscaler-amd64
docker buildx build -f Dockerfile.amd64 -t vmindtech/cluster-autoscaler:$1 --platform "linux/amd64" .
