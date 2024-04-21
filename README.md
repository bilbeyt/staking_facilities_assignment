# Staking Facilities Assignment

## Requirements and Frameworks

1. Go > 1.22
2. gin
3. godotenv
4. go-ethereum

## Design Choices

go-ethereum package is used for L1 operations and to interact with beacon API Web3Client is implemented.

For blockreward endpoint, etherscan block reward is calculated through having block number from slot id and then calculating 
transaction fees and burnt gas fee for block. In order to calculate if the block is `MEV` relayed, checked transaction base fee with a factor
as mev operators are paying much more to normal transactions to get priority.

To comply with the rate limit of 30 requests per second, implemented custom httpClient with rate.Limiter, as both L1 and beacon API
are using same endpoint, same http client is used for both.

## Usage

Dockerfile is provided for this assignment. You can use:

`docker build -t api .` on root folder.

Before running the container, create your own .env file using .env.template file. Then run:

`docker run -e ENV_PATH=${CONTAINER_ENV_PATH} -v ${LOCAL_ENV_PATH}:${CONTAINER_ENV_PATH} --net=host api`

I have used `--net=host` because without it, rpc was not working.

## Example Requests

### /blockreward Endpoint

1. `curl -X GET http://localhost:8080/blockreward/1`

    This will return `{"error":"Slot is missing"}`
2. `curl -X GET http://localhost:8080/blockreward/100000000`
    
    This will return  `{"error":"Slot is in the future"}`
3. `curl -X GET http://localhost:8080/blockreward/8886688`

    This will return `{"reward":"14173226.892490975","status":"vanilla"}`
4. `curl -X GET http://localhost:8080/blockreward/8886690`
    
    This will return `{"reward":"45486304.688277971","status":"mev"}`

### /syncduties Endpoint

1. `curl -X GET http://localhost:8080/syncduties/1`

   This will return `{"error":"Slot is missing"}`
2. `curl -X GET http://localhost:8080/syncduties/100000000`

   This will return  `{"error":"Slot is in the future"}`
3. `curl -X GET http://localhost:8080/syncduties/8886688`

   This will return list of public keys of validators who have a duty in sync committee for slot 8886688.

## Running Tests

You need local environment for this. Assuming you already have repo fork and go in your system.

1. go to /src
2. Install dependencies by `go mod download`
3. Run `go test .`