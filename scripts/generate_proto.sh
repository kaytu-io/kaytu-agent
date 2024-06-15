protoc --go_out=pkg/proto/src/golang --go_opt=paths=source_relative \
    --go-grpc_out=pkg/proto/src/golang --go-grpc_opt=paths=source_relative \
    pkg/proto/*.proto
mv pkg/proto/src/golang/pkg/proto/* pkg/proto/src/golang/
rm -rf pkg/proto/src/golang/pkg