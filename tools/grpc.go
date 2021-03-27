package tools

import (
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// IsGrpcNotFound returns true if the error is representing
// gRPC not found status code. false if not.
func IsGrpcNotFound(e error) bool {
	if st, ok := grpcstatus.FromError(e); ok && st.Code() == grpccodes.NotFound {
		return true
	}
	return false
}
