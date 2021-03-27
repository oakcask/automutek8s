package tools

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

const buckendBucketName string = "automutek8s-terraform-backend"

// TFObject is object in terraform json
type TFObject map[string]interface{}

// TFDocument corresponds to terraform source file we are going to
// generate
type TFDocument struct {
	Terraform TFTerraform           `json:"terraform"`
	Variable  map[string]TFVariable `json:"variable"`
}

// TFTerraformBackend is backend configuration using GCS.
type TFTerraformBackend struct {
	GCS TFObject `json:"gcs"`
}

// TFTerraform is "terraform" block in terraform source file.
type TFTerraform struct {
	Backend TFTerraformBackend `json:"backend"`
}

// TFVariable is "variable" block in terraform source file.
type TFVariable struct {
	Default interface{} `json:"default"`
}

// CreateOrGetTFBackendBucket generates Cloud Storage bucket name for
// terraform backend and create new bucket with that name if it does not exists.
func CreateOrGetTFBackendBucket(ctx context.Context, client *storage.Client) (string, error) {
	projectID, e := GetProjectID(ctx)
	if e != nil {
		return "", e
	}
	itr := client.Buckets(ctx, projectID)
	if e != nil {
		return "", e
	}

	for {
		battrs, e := itr.Next()
		if e == iterator.Done {
			break
		}
		if e != nil {
			return "", e
		}

		if _, ok := battrs.Labels[buckendBucketName]; ok {
			log.Printf("using existing backend bucket: %s", battrs.Name)
			return battrs.Name, nil
		}
	}

	bucketAttrs := &storage.BucketAttrs{
		Labels: map[string]string{},
	}
	bucketAttrs.Labels[buckendBucketName] = ""
	random, e := uuid.NewRandom()
	if e != nil {
		return "", e
	}
	bucketName := fmt.Sprintf("tfstate-%s", random.String())

	e = client.Bucket(bucketName).Create(ctx, projectID, bucketAttrs)
	if e != nil {
		return "", e
	}

	log.Printf("bucket for backend created: %s", bucketName)
	return bucketName, nil
}
