/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

var ErrStore = errors.New("error")

func ErrGenericError(text string) error {
	return fmt.Errorf("store %w : %s", ErrStore, text)
}

func ErrGenericErrorWrap(text string, err error) error {
	return fmt.Errorf("store %s : %w", text, err)
}

const (
	CHECKPOINT_PATH = "checkpoint/latest"
)

// Checkpoint tracks the last successfully processed event in a partition.
type Checkpoint struct {
	Region string
	UUID   string
}

var regionRegex = regexp.MustCompile("[^/]+?$")

type BlobStore struct {
	cc *container.Client
}

func NewBlobStore(containerClient *container.Client) (*BlobStore, error) {
	return &BlobStore{
		cc: containerClient,
	}, nil
}

func (b *BlobStore) SetCheckpoint(ctx context.Context, checkpoint Checkpoint) (*time.Time, azcore.ETag, error) {
	blobName, err := nameForCheckpoint()
	if err != nil {
		return &time.Time{}, "", ErrGenericErrorWrap("setting checkpoint", err)
	}

	return b.setCheckpointMetadata(ctx, blobName)
}

func nameForCheckpoint() (string, error) {
	return CHECKPOINT_PATH, nil
}

func (b *BlobStore) setCheckpointMetadata(ctx context.Context, blobName string) (*time.Time, azcore.ETag, error) {
	blobMetadata := newCheckpointMetadata()
	blobClient := b.cc.NewBlockBlobClient(blobName)

	setMetadataResp, err := blobClient.SetMetadata(ctx, blobMetadata, nil)
	if err == nil {
		return setMetadataResp.LastModified, *setMetadataResp.ETag, nil
	}

	if !bloberror.HasCode(err, bloberror.BlobNotFound) {
		return nil, "", ErrGenericErrorWrap("setting checkpoint metadata", err)
	}

	uploadResp, err := blobClient.Upload(ctx, streaming.NopCloser(bytes.NewReader([]byte{})), &blockblob.UploadOptions{
		Metadata: blobMetadata,
	})

	if err != nil {
		return nil, "", ErrGenericErrorWrap("uploading metadata to blob", err)
	}

	return uploadResp.LastModified, *uploadResp.ETag, nil
}

func newCheckpointMetadata() map[string]*string {
	m := map[string]*string{}
	return m
}

func (b *BlobStore) GetCheckpoint(ctx context.Context) ([]Checkpoint, error) {
	var checkpoints []Checkpoint

	prefix := CHECKPOINT_PATH
	pager := b.cc.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
		Prefix: &prefix,
		Include: container.ListBlobsInclude{
			Metadata: true,
		},
	})

	for pager.More() {
		resp, err := pager.NextPage(ctx)

		if err != nil {
			return nil, ErrGenericErrorWrap("getting next list of pages for checkpoint", err)
		}

		for _, blob := range resp.Segment.BlobItems {
			checkpoints = append(checkpoints, Checkpoint{
				Region: regionRegex.FindString(*blob.Name),
			})
		}
	}

	return checkpoints, nil
}

func (b *BlobStore) GetCheckpointLastModify(ctx context.Context) (time.Time, error) {
	prefix := CHECKPOINT_PATH
	pager := b.cc.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
		Prefix: &prefix,
		Include: container.ListBlobsInclude{
			Metadata: true,
		},
	})

	for pager.More() {
		resp, err := pager.NextPage(ctx)

		if err != nil {
			return time.Time{}, ErrGenericErrorWrap("getting next list of pages for last modify ", err)
		}

		for _, blob := range resp.Segment.BlobItems {
			return *blob.Properties.LastModified, nil
		}
	}

	return time.Time{}, nil
}
