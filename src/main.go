package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"os"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Msg("Error loading .env file")
	}
	rpcURL := os.Getenv("RPC_URL")
	parsedUrl, err := url.Parse(rpcURL)
	if err != nil {
		log.Fatal().Msg("Can not parse the rpc url")
	}
	client := Web3Client{BaseUrl: *parsedUrl}

	router := gin.Default()
	router.GET("/blockreward/:slotId", GetBlockRewardHandler(client))
	router.GET("/syncduties/:slotId", GetSyncDutiesHandler(client))

	err = router.Run(":8080")
	if err != nil {
		log.Err(err).Msg("Server exit")
	}
}

func GetBlockRewardHandler(client Web3Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		slotId := c.Param("slotId")
		reward, status, err := client.GetBlockRewardAndStatusBySlot(c, slotId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"reward": reward,
			"status": status,
		})
	}
}

func GetSyncDutiesHandler(client Web3Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		slotId := c.Param("slotId")
		pubKeys, err := client.GetSyncCommitteeDuties(slotId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, nil)
			return
		}
		c.JSON(http.StatusOK, pubKeys)
	}
}
