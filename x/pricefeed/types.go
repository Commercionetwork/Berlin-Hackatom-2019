package pricefeed

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Asset struct that represents an asset in the pricefeed
type Asset struct {
	Type        string `json:"type"`        // Either nft or ft
	AssetCode   string `json:"asset_code"`  // The nft id, otherwise empty
	AssetName   string `json:"asset_name"`  // Either the ft name or nft name
	Description string `json:"description"` // The asset description
}

// Oracle struct that documents which address an oracle is using
type Oracle struct {
	OracleAddress string `json:"oracle_address"`
}

// CurrentPrice struct that contains the metadata of a current price for a particular asset in the pricefeed module.
type CurrentPrice struct {
	AssetName string  `json:"asset_name"`
	AssetCode string  `json:"asset_code"`
	Price     sdk.Dec `json:"price"`
	Expiry    sdk.Int `json:"expiry"`
}

// PostedPrice struct represented a price for an asset posted by a specific oracle
type PostedPrice struct {
	AssetName     string  `json:"asset_name"`
	AssetCode     string  `json:"asset_code"`
	OracleAddress string  `json:"oracle_address"`
	Price         sdk.Dec `json:"price"`
	Expiry        sdk.Int `json:"expiry"`
}

// PendingPriceAsset struct that contains the info about the asset which price is still to be determined
type PendingPriceAsset struct {
	AssetName     string `json:"asset_name"`
	AssetCode     string `json:"asset_code"`
}

// SortDecs provides the interface needed to sort sdk.Dec slices
type SortDecs []sdk.Dec

func (a SortDecs) Len() int           { return len(a) }
func (a SortDecs) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SortDecs) Less(i, j int) bool { return a[i].LT(a[j]) }
