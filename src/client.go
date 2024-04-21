package main

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const BlockDetailPath = "/eth/v2/beacon/blocks/"
const StatePath = "/eth/v1/beacon/states/"

var GWEI = big.NewInt(1000000000)

type Web3Client struct {
	BaseUrl url.URL
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

func sendAPIRequest(requestUrl string, requestName string, v interface{}) error {
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		log.Info().Err(err).Str("requestName", requestName).Msg("can not create request")
		return err
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Info().Err(err).Str("requestName", requestName).Msg("can not send request")
		return err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil {
			return err
		}
		time.Sleep(time.Duration(retryAfter) * time.Second)
		return sendAPIRequest(requestUrl, requestName, v)
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

	if err = json.Unmarshal(body, v); err != nil {
		return err
	}

	return nil
}

func (c *Web3Client) getBlockHash(slotId string) (common.Hash, error) {
	endpoint := c.BaseUrl.String() + BlockDetailPath + slotId
	var blockDetail beaconBlockDetailResponse
	err := sendAPIRequest(endpoint, "beacon block detail", &blockDetail)
	if err != nil {
		return common.Hash{}, err
	}
	blockHash := blockDetail.Data.Message.Body.ExecutionPayload.BlockHash
	return common.HexToHash(blockHash), nil
}

func (c *Web3Client) getSyncCommitteesValidatorIndexes(slotId string) ([]string, error) {
	endpoint := c.BaseUrl.String() + StatePath + slotId + "/sync_committees"
	var response syncCommitteesResponse
	err := sendAPIRequest(endpoint, "sync committees", &response)
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
	err := sendAPIRequest(endpoint, "receive pubkeys of validators", &response)
	if err != nil {
		return nil, err
	}
	var pubKeys []string
	for _, info := range response.Data {
		pubKeys = append(pubKeys, info.Validator.Pubkey)
	}
	return pubKeys, nil
}

func (c *Web3Client) GetBlockRewardAndStatusBySlot(ctx context.Context, slotId string) (string, string, error) {
	blockHash, err := c.getBlockHash(slotId)
	if err != nil {
		return "nil", "", err
	}
	client, err := ethclient.Dial(c.BaseUrl.String())
	if err != nil {
		log.Info().Err(err).Msg("can not dial ethereum client")
		return "nil", "", err
	}
	block, err := client.BlockByHash(ctx, blockHash)
	if err != nil {
		log.Info().Err(err).Msg("can not get block by hash")
		return "nil", "", err
	}
	burntFees := new(big.Int).Mul(block.BaseFee(), big.NewInt(int64(block.GasUsed())))
	txCosts := new(big.Int).SetInt64(0)
	status := "vanilla"
	for _, tx := range block.Transactions() {
		receipt, err := client.TransactionReceipt(ctx, tx.Hash())
		cost := tx.Cost()
		gasPrice := tx.GasPrice()
		if err == nil {
			cost = new(big.Int).Mul(receipt.EffectiveGasPrice, big.NewInt(int64(receipt.GasUsed)))
			gasPrice = receipt.EffectiveGasPrice
		}
		if gasPrice.Cmp(new(big.Int).Mul(block.BaseFee(), big.NewInt(3))) == 1 {
			status = "mev"
		}
		txCosts = new(big.Int).Add(txCosts, cost)
	}

	reward := new(big.Int).Sub(txCosts, burntFees)
	rewardAsFloat := new(big.Float).Quo(new(big.Float).SetInt(reward), new(big.Float).SetInt(GWEI))
	return rewardAsFloat.Text('f', 9), status, nil
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
