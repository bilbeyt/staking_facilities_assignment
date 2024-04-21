package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
	"io"
	"math/big"
	"net/http"
	"net/url"
)

const BlockDetailPath = "/eth/v2/beacon/blocks/"
const StatePath = "/eth/v1/beacon/states/"
const MevFeeCalculationFactor = 3

var BlocksAvailableAfterSlot = big.NewInt(4700012) // Paris merge is on 4700013
var GWEI = big.NewInt(1000000000)

type SlotMissingError struct {
	msg string
}

func (e *SlotMissingError) Error() string {
	return e.msg
}

type FutureSlotError struct {
	msg string
}

func (e *FutureSlotError) Error() string {
	return e.msg
}

type Web3Client struct {
	BaseUrl    *url.URL
	httpClient *http.Client
	w3Client   *ethclient.Client
}

func NewWeb3Client(baseUrl *url.URL, reqPerSec rate.Limit) *Web3Client {
	limiter := rate.NewLimiter(reqPerSec, 1)
	httpClient := &http.Client{
		Transport: &rateLimitTransport{
			rateLimiter: limiter,
			transport:   http.DefaultTransport,
		},
	}
	rpcClient, err := rpc.DialOptions(context.Background(), baseUrl.String(), rpc.WithHTTPClient(httpClient))
	if err != nil {
		log.Info().Err(err).Msg("can not dial ethereum client")
		return nil
	}
	client := ethclient.NewClient(rpcClient)
	return &Web3Client{BaseUrl: baseUrl, httpClient: httpClient, w3Client: client}
}

type beaconBlockDetailResponse struct {
	Data struct {
		Message struct {
			Body struct {
				ExecutionPayload struct {
					BlockHash string `json:"block_hash"`
				} `json:"execution_payload"`
			} `json:"body"`
		} `json:"message"`
	} `json:"data"`
}

type syncCommitteesResponse struct {
	Data struct {
		Validators []string `json:"validators"`
	} `json:"data"`
}

type validatorsDetailResponse struct {
	Data []struct {
		Validator struct {
			Pubkey string `json:"pubkey"`
		} `json:"validator"`
	} `json:"data"`
}

type BeaconHeader struct {
	Data []struct {
		Header struct {
			Message struct {
				Slot string `json:"slot"`
			} `json:"message"`
		} `json:"header"`
	} `json:"data"`
}

type rateLimitTransport struct {
	rateLimiter *rate.Limiter
	transport   http.RoundTripper
}

func (rlt *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	err := rlt.rateLimiter.Wait(context.Background())
	if err != nil {
		return nil, err
	}
	return rlt.transport.RoundTrip(req)
}

func (c *Web3Client) sendAPIRequest(requestUrl string, requestName string, v interface{}) error {
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		log.Info().Err(err).Str("requestName", requestName).Msg("can not create request")
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Info().Err(err).Str("requestName", requestName).Msg("can not send request")
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Info().Err(err).Str("requestName", requestName).Msg("can not close beacon block detail body")
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Info().Err(err).Str("requestName", requestName).Msg("can not read beacon block detail response")
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		return &FutureSlotError{msg: "Slot is in the future"}
	}

	if resp.StatusCode == http.StatusBadRequest {
		return &SlotMissingError{msg: "Slot is not found"}
	}

	if err = json.Unmarshal(body, v); err != nil {
		return err
	}

	return nil
}

func (c *Web3Client) getBlockHash(slotId string) (common.Hash, error) {
	endpoint := c.BaseUrl.String() + BlockDetailPath + slotId
	var blockDetail beaconBlockDetailResponse
	err := c.sendAPIRequest(endpoint, "beacon block detail", &blockDetail)
	if err != nil {
		return common.Hash{}, err
	}
	blockHash := blockDetail.Data.Message.Body.ExecutionPayload.BlockHash
	return common.HexToHash(blockHash), nil
}

func (c *Web3Client) getSyncCommitteesValidatorIndexes(slotId string) ([]string, error) {
	endpoint := c.BaseUrl.String() + StatePath + slotId + "/sync_committees"
	var response syncCommitteesResponse
	err := c.sendAPIRequest(endpoint, "sync committees", &response)
	if err != nil {
		return nil, err
	}
	return response.Data.Validators, nil
}

func (c *Web3Client) getPubKeysOfSyncCommittees(slotId string, validatorIndexes []string) ([]string, error) {
	endpoint := c.BaseUrl.String() + StatePath + slotId + "/validators"
	for index, validatorIndex := range validatorIndexes {
		if index == 0 {
			endpoint += "?"
		}
		endpoint += "id=" + validatorIndex
		if index != len(validatorIndexes)-1 {
			endpoint += "&"
		}
	}
	var response validatorsDetailResponse
	err := c.sendAPIRequest(endpoint, "receive pubkeys of validators", &response)
	if err != nil {
		return nil, err
	}
	var pubKeys []string
	for _, info := range response.Data {
		pubKeys = append(pubKeys, info.Validator.Pubkey)
	}
	return pubKeys, nil
}

func (c *Web3Client) getCurrentSlotId() *big.Int {
	slotIdEndpoint := c.BaseUrl.String() + "/eth/v1/beacon/headers"
	var header BeaconHeader
	err := c.sendAPIRequest(slotIdEndpoint, "current slot id", &header)
	if err != nil {
		return big.NewInt(0)
	}
	slotAsInt, ok := new(big.Int).SetString(header.Data[0].Header.Message.Slot, 10)
	if !ok {
		return big.NewInt(0)
	}
	return slotAsInt
}

func (c *Web3Client) GetBlockRewardAndStatusBySlot(ctx context.Context, slotId string) (*string, *string, error) {
	slotIdAsInt, ok := new(big.Int).SetString(slotId, 10)
	if !ok {
		return nil, nil, errors.New("can not convert slotId to bigInt")
	}
	if slotIdAsInt.Cmp(BlocksAvailableAfterSlot) != 1 {
		return nil, nil, &SlotMissingError{msg: "Slot is missing"}
	}
	currentSlotId := c.getCurrentSlotId()
	if slotIdAsInt.Cmp(currentSlotId) == 1 {
		return nil, nil, &FutureSlotError{msg: "Slot is in the future"}
	}
	blockHash, err := c.getBlockHash(slotId)
	if err != nil {
		return nil, nil, err
	}

	block, err := c.w3Client.BlockByHash(ctx, blockHash)
	if err != nil {
		log.Info().Err(err).Msg("can not get block by hash")
		return nil, nil, err
	}
	burntFees := new(big.Int).Mul(block.BaseFee(), big.NewInt(int64(block.GasUsed())))
	txCosts := new(big.Int).SetInt64(0)
	status := "vanilla"
	for _, tx := range block.Transactions() {
		receipt, err := c.w3Client.TransactionReceipt(ctx, tx.Hash())
		cost := tx.Cost()
		gasPrice := tx.GasPrice()
		if err == nil {
			cost = new(big.Int).Mul(receipt.EffectiveGasPrice, big.NewInt(int64(receipt.GasUsed)))
			gasPrice = receipt.EffectiveGasPrice
		}
		if gasPrice.Cmp(new(big.Int).Mul(block.BaseFee(), big.NewInt(MevFeeCalculationFactor))) == 1 {
			status = "mev"
		}
		txCosts = new(big.Int).Add(txCosts, cost)
	}

	reward := new(big.Int).Sub(txCosts, burntFees)
	rewardAsFloat := new(big.Float).Quo(new(big.Float).SetInt(reward), new(big.Float).SetInt(GWEI))
	rewardAsText := rewardAsFloat.Text('f', 9)
	return &rewardAsText, &status, nil
}

func (c *Web3Client) GetSyncCommitteeDuties(slotId string) ([]string, error) {
	validatorIndexes, err := c.getSyncCommitteesValidatorIndexes(slotId)
	if err != nil {
		return nil, err
	}

	pubKeys, err := c.getPubKeysOfSyncCommittees(slotId, validatorIndexes)
	if err != nil {
		return nil, err
	}
	return pubKeys, nil
}
