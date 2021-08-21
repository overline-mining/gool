package main

import (
  "context"
  "flag"
  "time"

  "github.com/wavesplatform/gowaves/pkg/util/fdlimit"
  "github.com/wavesplatform/gowaves/pkg/libs/ntptime"
  "github.com/wavesplatform/gowaves/pkg/types"
  
  "github.com/overline-mining/gool/src/common"

  "go.uber.org/zap"
)

var (
  logLevel = flag.String("log-level", "INFO", "Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
  blockchainType = flag.String("blockchain-type", "mainnet", "Blockchain type: mainnet/testnet/integration")
  limitAllConnections = flag.Uint("limit-connections", 60, "Total limit of network connections, both inbound and outbound. Divided in half to limit each direction. Default value is 60.")
  dbFileDescriptors = flag.Int("db-file-descriptors", 500, "Maximum allowed file descriptors count that will be used by state database. Default value is 500.")
)

func main() {
  flag.Parse()

  maxFDs, err := fdlimit.MaxFDs()
	if err != nil {
		zap.S().Fatalf("Initialization failure: %v", err)
	}
	_, err = fdlimit.RaiseMaxFDs(maxFDs)
	if err != nil {
		zap.S().Fatalf("Initialization failure: %v", err)
	}
	if maxAvailableFileDescriptors := int(maxFDs) - int(*limitAllConnections) - 10; *dbFileDescriptors > maxAvailableFileDescriptors {
		zap.S().Fatalf("Invalid 'db-file-descriptors' flag value (%d). Value shall be less or equal to %d.", *dbFileDescriptors, maxAvailableFileDescriptors)
	}

  common.SetupLogger(*logLevel)

  ctx, cancel := context.WithCancel(context.Background())

  ntpTime, err := getNtp(ctx)
	if err != nil {
		zap.S().Error(err)
		cancel()
		return
	}

  zap.S().Info(ntpTime)
}

func getNtp(ctx context.Context) (types.Time, error) {
	if *blockchainType == "integration" {
		return ntptime.Stub{}, nil
	}
	tm, err := ntptime.TryNew("pool.ntp.org", 10)
	if err != nil {
		return nil, err
	}
	go tm.Run(ctx, 2*time.Minute)
	return tm, nil
}