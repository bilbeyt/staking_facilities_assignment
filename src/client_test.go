package main_test

import (
	"context"
	"encoding/json"
	src "github.com/bilbeyt/staking_facilities_assignment"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type RequestBody struct {
	Method string `json:"method"`
}

func setupServer(testKey string) *httptest.Server {
	r := mux.NewRouter()
	testData := src.AllTestData[testKey]

	r.HandleFunc("/eth/v1/beacon/headers", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(testData.HeadersStatusCode)
		_, _ = rw.Write([]byte(testData.HeadersResponse))
	})
	r.HandleFunc("/eth/v2/beacon/blocks/{slotId}", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(testData.BlocksStatusCode)
		_, _ = rw.Write([]byte(testData.BlocksResponse))
	})
	r.HandleFunc("/eth/v1/beacon/states/{slotId}/sync_committees", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(testData.SyncCommitteesStatusCode)
		_, _ = rw.Write([]byte(testData.SyncCommitteesResponse))
	})
	r.HandleFunc("/eth/v1/beacon/states/{slotId}/validators", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(testData.SyncCommitteesDetailStatusCode)
		_, _ = rw.Write([]byte(testData.SyncCommitteesDetailResponse))
	})
	r.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return
		}
		var requestBody RequestBody
		err = json.Unmarshal(body, &requestBody)
		if err != nil {
			return
		}
		rw.WriteHeader(http.StatusOK)
		switch requestBody.Method {
		case "eth_getBlockByHash":
			_, _ = rw.Write([]byte(testData.BlockHashResponse))
		case "eth_getTransactionReceipt":
			_, _ = rw.Write([]byte(testData.TransactionReceiptResponse))
		default:
			return
		}
	})
	server := httptest.NewServer(r)
	return server
}

func TestGetBlockRewardAndStatusBySlotMev(t *testing.T) {
	server := setupServer("mev")
	defer server.Close()
	parsedUrl, _ := url.Parse(server.URL)
	client := src.NewWeb3Client(parsedUrl, 1)
	ctx := context.Background()
	reward, status, err := client.GetBlockRewardAndStatusBySlot(ctx, "4700013")
	if err != nil {
		t.Fail()
	}
	if *reward != "0.000000002" {
		t.Errorf("Expected reward to be 0.000000002, but got %s", *reward)
	}
	if *status != "mev" {
		t.Errorf("Expected status to be mev, but got %s", *status)
	}
}

func TestGetBlockRewardAndStatusBySlotVanilla(t *testing.T) {
	server := setupServer("vanilla")
	defer server.Close()
	parsedUrl, _ := url.Parse(server.URL)
	client := src.NewWeb3Client(parsedUrl, 1)
	ctx := context.Background()
	reward, status, err := client.GetBlockRewardAndStatusBySlot(ctx, "4700013")
	if err != nil {
		t.Fail()
	}
	if *reward != "0.000000001" {
		t.Errorf("Expected reward to be 0.000000001, but got %s", *reward)
	}
	if *status != "vanilla" {
		t.Errorf("Expected status to be mev, but got %s", *status)
	}
}

func TestGetBlockRewardAndStatusMissingSlot(t *testing.T) {
	server := setupServer("rewardMissingSlot")
	defer server.Close()
	parsedUrl, _ := url.Parse(server.URL)
	client := src.NewWeb3Client(parsedUrl, 1)
	ctx := context.Background()
	reward, status, err := client.GetBlockRewardAndStatusBySlot(ctx, "5")
	if status != nil || reward != nil {
		t.Fail()
	}
	if err.Error() != "Slot is missing" {
		t.Fail()
	}
}

func TestGetBlockRewardAndStatusFutureSlot(t *testing.T) {
	server := setupServer("rewardFutureSlot")
	defer server.Close()
	parsedUrl, _ := url.Parse(server.URL)
	client := src.NewWeb3Client(parsedUrl, 1)
	ctx := context.Background()
	reward, status, err := client.GetBlockRewardAndStatusBySlot(ctx, "100000000000")
	if status != nil || reward != nil {
		t.Fail()
	}
	if err.Error() != "Slot is in the future" {
		t.Fail()
	}
}

func TestSyncDuties(t *testing.T) {
	server := setupServer("syncDuties")
	defer server.Close()
	parsedUrl, _ := url.Parse(server.URL)
	client := src.NewWeb3Client(parsedUrl, 1)
	keys, err := client.GetSyncCommitteeDuties("100000000000")
	if len(keys) == 1 && keys[0] != "0x0000000000000000000000000000000000000000000000000000000000000001" {
		t.Fail()
	}
	if err != nil {
		t.Fail()
	}
}

func TestSyncDutiesMissingSlot(t *testing.T) {
	server := setupServer("syncMissingSlot")
	defer server.Close()
	parsedUrl, _ := url.Parse(server.URL)
	client := src.NewWeb3Client(parsedUrl, 1)
	keys, err := client.GetSyncCommitteeDuties("10")
	if keys != nil {
		t.Fail()
	}
	if err.Error() != "Slot is not found" {
		t.Fail()
	}
}

func TestSyncDutiesFutureSlot(t *testing.T) {
	server := setupServer("syncFutureSlot")
	defer server.Close()
	parsedUrl, _ := url.Parse(server.URL)
	client := src.NewWeb3Client(parsedUrl, 1)
	keys, err := client.GetSyncCommitteeDuties("100000000000")
	if keys != nil {
		t.Fail()
	}
	if err.Error() != "Slot is in the future" {
		t.Fail()
	}
}
