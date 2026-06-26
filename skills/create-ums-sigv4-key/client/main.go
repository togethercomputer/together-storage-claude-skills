package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	pb "github.com/togethercomputer/user-management-service/clients/go/gen/ums"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	endpoint := os.Getenv("UMS_ENDPOINT") // e.g. ums.example.com:50051
	serviceKey := os.Getenv("UMS_SERVICE_KEY")
	existing := os.Getenv("EXISTING_KEY")

	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	if err != nil {
		fmt.Println("dial err:", err)
		os.Exit(1)
	}
	defer conn.Close()
	cl := pb.NewKeyClient(conn)
	ctx, cancel := context.WithTimeout(metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+serviceKey), 20*time.Second)
	defer cancel()

	k, err := cl.GetV1(ctx, &pb.KeyGetV1_Request{KeyId: existing})
	if err != nil {
		fmt.Println("GetV1 err:", err)
		os.Exit(1)
	}
	fmt.Printf("EXISTING: project=%s creator=%s tokenType=%v scopes=%v\n", k.GetProjectId(), k.GetCreatorId(), k.GetTokenType(), k.GetScopes())

	scopes := &pb.KeyScopesV1{Tcloud: true}
	exp := timestamppb.New(time.Now().Add(365 * 24 * time.Hour))
	tt := pb.KeyTokenType_KEY_TOKEN_TYPE_SIGV4
	rep, err := cl.CreateProjectKeyV1(ctx, &pb.KeyCreateProjectKeyV1_Request{
		ProjectId:   k.GetProjectId(),
		CreatorId:   k.GetCreatorId(),
		Description: "t4-ceph-e2e deploy verification key",
		ExpiresAt:   exp,
		Scopes:      scopes,
		TokenType:   &tt,
	})
	if err != nil {
		fmt.Println("CreateProjectKeyV1 err:", err)
		os.Exit(1)
	}
	fmt.Println("=== NEW KEY ===")
	fmt.Println("AWS_ACCESS_KEY_ID=" + rep.GetId())
	fmt.Println("AWS_SECRET_ACCESS_KEY=" + rep.GetToken())
	fmt.Printf("tokenType=%v expiresAt=%s suffix=%s\n", rep.GetTokenType(), rep.GetExpiresAt().AsTime().Format(time.RFC3339), rep.GetTokenSuffix())
}
