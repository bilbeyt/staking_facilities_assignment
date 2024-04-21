package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func main() {
	envFilePath := os.Getenv("ENV_PATH")
	err := godotenv.Load(envFilePath)
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading .env file")
	}
	gin.SetMode(os.Getenv("GIN_MODE"))
	rpcURL := os.Getenv("RPC_URL")
	parsedUrl, err := url.Parse(rpcURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Can not parse the rpc url")
	}

	rpcRateLimit := os.Getenv("RPC_RATE_LIMIT")
	serverAddr := os.Getenv("SERVER_ADDR")
	rpcRateLimitFloat, err := strconv.ParseFloat(rpcRateLimit, 10)
	trustedProxiesStr := os.Getenv("TRUSTED_PROXIES")
	if err != nil {
		log.Fatal().Err(err).Msg("can not parse rpc rate limit")
	}
	client := NewWeb3Client(parsedUrl, rate.Limit(rpcRateLimitFloat))

	router := gin.Default()
	router.ForwardedByClientIP = true
	err = router.SetTrustedProxies(strings.Split(trustedProxiesStr, ","))
	if err != nil {
		log.Fatal().Err(err).Msg("can not set trusted proxies")
	}
	router.GET("/blockreward/:slotId", GetBlockRewardHandler(client))
	router.GET("/syncduties/:slotId", GetSyncDutiesHandler(client))

	err = router.Run(serverAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("Server exit")
	}
}

func GetBlockRewardHandler(client *Web3Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		slotId := c.Param("slotId")
		reward, status, err := client.GetBlockRewardAndStatusBySlot(c, slotId)
		if err != nil {
			var slotMissingError *SlotMissingError
			var futureSlotError *FutureSlotError
			if errors.As(err, &slotMissingError) {
				c.JSON(http.StatusNotFound, gin.H{
					"error": err.Error(),
				})
				return
			}
			if errors.As(err, &futureSlotError) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"reward": reward,
			"status": status,
		})
	}
}

func GetSyncDutiesHandler(client *Web3Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		slotId := c.Param("slotId")
		pubKeys, err := client.GetSyncCommitteeDuties(slotId)
		if err != nil {
			var slotMissingError *SlotMissingError
			var futureSlotError *FutureSlotError
			if errors.As(err, &slotMissingError) {
				c.JSON(http.StatusNotFound, gin.H{
					"error": err.Error(),
				})
				return
			}
			if errors.As(err, &futureSlotError) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		c.JSON(http.StatusOK, pubKeys)
	}
}
