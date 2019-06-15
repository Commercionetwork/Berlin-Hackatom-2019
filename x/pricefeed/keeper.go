package pricefeed

import (
	"errors"
	"fmt"
	"github.com/kava-labs/kava-devnet/blockchain/x/cdp"
	"github.com/kava-labs/kava-devnet/blockchain/x/cdp/client"
	"sort"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TODO refactor constants to app.go
const (
	// ModuleKey is the name of the module
	ModuleName = "pricefeed"

	// StoreKey is the store key string for gov
	StoreKey = ModuleName

	// RouterKey is the message route for gov
	RouterKey = ModuleName

	// QuerierRoute is the querier route for gov
	QuerierRoute = ModuleName

	// Parameter store default namestore
	DefaultParamspace = ModuleName

	// Store prefix for the raw pricefeed of an asset
	RawPriceFeedPrefix = StoreKey + ":raw:"

	// Store prefix for the current price of an asset
	CurrentPricePrefix = StoreKey + ":currentprice:"

	// Store Prefix for the assets in the pricefeed system
	AssetPrefix = StoreKey + ":assets"

	// OraclePrefix store prefix for the oracle accounts
	OraclePrefix = StoreKey + ":oracles"

	// EstimableAssetPrefix store prefix for the estimable assets
	EstimableAssetPrefix = StoreKey + ":estimableassets"
)

// Keeper struct for pricefeed module
type Keeper struct {
	cdp           cdp.Keeper
	priceStoreKey sdk.StoreKey
	cdc           *codec.Codec
	codespace     sdk.CodespaceType
}

// NewKeeper returns a new keeper for the pricefeed modle
func NewKeeper(storeKey sdk.StoreKey, cdc *codec.Codec, codespace sdk.CodespaceType) Keeper {
	return Keeper{
		priceStoreKey: storeKey,
		cdc:           cdc,
		codespace:     codespace,
	}
}

// AddOracle adds an Oracle to the store
func (k Keeper) AddOracle(ctx sdk.Context, address string) {

	oracles := k.GetOracles(ctx)
	oracles = append(oracles, Oracle{OracleAddress: address})
	store := ctx.KVStore(k.priceStoreKey)
	store.Set(
		[]byte(OraclePrefix), k.cdc.MustMarshalBinaryBare(oracles),
	)
}

// AddAsset adds an asset to the store
func (k Keeper) AddAsset(ctx sdk.Context, assetCode string, desc string) {
	assets := k.GetAssets(ctx)
	assets = append(assets, Asset{AssetCode: assetCode, Description: desc})
	store := ctx.KVStore(k.priceStoreKey)
	store.Set([]byte(AssetPrefix), k.cdc.MustMarshalBinaryBare(assets))
}

/*
SetPrice deve essere modificato perché non è sufficiente l'assetCode
Nel caso di NFT deve minimo essere passato in maniera separate il tipo di NFT e l'ID


*/
// SetPrice updates the posted price for a specific oracle
func (k Keeper) SetPrice(ctx sdk.Context, oracle sdk.AccAddress, assetCode string, price sdk.Dec, expiry sdk.Int) (PostedPrice, sdk.Error) {
	// If the expiry is less than or equal to the current blockheight, we consider the price valid
	if expiry.GTE(sdk.NewInt(ctx.BlockHeight())) {
		store := ctx.KVStore(k.priceStoreKey)
		prices := k.GetRawPrices(ctx, assetCode)
		var index int
		found := false
		for i := range prices {
			if prices[i].OracleAddress == oracle.String() {
				index = i
				found = true
				break
			}
		}
		// set the price for that particular oracle
		if found {
			prices[index] = PostedPrice{AssetCode: assetCode, OracleAddress: oracle.String(), Price: price, Expiry: expiry}
		} else {
			prices = append(prices, PostedPrice{assetCode, oracle.String(), price, expiry})
			index = len(prices) - 1
		}

		store.Set([]byte(RawPriceFeedPrefix+assetCode), k.cdc.MustMarshalBinaryBare(prices))

		return prices[index], nil
	}

	return PostedPrice{}, ErrExpired(k.codespace)

}

// SetCurrentPrices updates the price of an asset to the median of all valid oracle inputs
func (k Keeper) SetCurrentPrices(ctx sdk.Context) sdk.Error {
	assets := k.GetAssets(ctx)
	for _, v := range assets {
		// NON è sufficiente per l'NFT
		assetCode := v.AssetCode
		prices := k.GetRawPrices(ctx, assetCode)
		var notExpiredPrices []CurrentPrice
		// filter out expired prices
		for _, v := range prices {
			if v.Expiry.GTE(sdk.NewInt(ctx.BlockHeight())) {
				notExpiredPrices = append(notExpiredPrices, CurrentPrice{
					AssetCode: v.AssetCode,
					Price:     v.Price,
					Expiry:    v.Expiry,
				})
			}
		}
		l := len(notExpiredPrices)
		var medianPrice sdk.Dec
		var expiry sdk.Int
		// TODO make threshold for acceptance (ie. require 51% of oracles to have posted valid prices
		if l == 0 {
			// Error if there are no valid prices in the raw pricefeed
			// return ErrNoValidPrice(k.codespace)
			medianPrice = sdk.NewDec(0)
			expiry = sdk.NewInt(0)
		} else if l == 1 {
			// Return immediately if there's only one price
			medianPrice = notExpiredPrices[0].Price
			expiry = notExpiredPrices[0].Expiry
		} else {
			// sort the prices
			sort.Slice(notExpiredPrices, func(i, j int) bool {
				return notExpiredPrices[i].Price.LT(notExpiredPrices[j].Price)
			})
			// If there's an even number of prices
			if l%2 == 0 {
				// TODO make sure this is safe.
				// Since it's a price and not a blance, division with precision loss is OK.
				price1 := notExpiredPrices[l/2-1].Price
				price2 := notExpiredPrices[l/2].Price
				sum := price1.Add(price2)
				divsor, _ := sdk.NewDecFromStr("2")
				medianPrice = sum.Quo(divsor)
				// TODO Check if safe, makes sense
				// Takes the average of the two expiries rounded down to the nearest Int.
				expiry = notExpiredPrices[l/2-1].Expiry.Add(notExpiredPrices[l/2].Expiry).Quo(sdk.NewInt(2))
			} else {
				// integer division, so we'll get an integer back, rounded down
				medianPrice = notExpiredPrices[l/2].Price
				expiry = notExpiredPrices[l/2].Expiry
			}
		}

		store := ctx.KVStore(k.priceStoreKey)
		currentPrice := CurrentPrice{
			AssetCode: assetCode,
			Price:     medianPrice,
			Expiry:    expiry,
		}
		store.Set(
			[]byte(CurrentPricePrefix+assetCode), k.cdc.MustMarshalBinaryBare(currentPrice),
		)
	}

	return nil
}

// GetOracles returns the oracles in the pricefeed store
func (k Keeper) GetEstimableAssets(ctx sdk.Context) []EstimableAsset {
	store := ctx.KVStore(k.priceStoreKey)
	//todo not sure of passing estimableAssPrefixx to get
	bz := store.Get([]byte(EstimableAssetPrefix))
	var estimableAssets []EstimableAsset
	k.cdc.MustUnmarshalBinaryBare(bz, &estimableAssets)
	return estimableAssets
}

// GetOracles returns the oracles in the pricefeed store
func (k Keeper) GetOracles(ctx sdk.Context) []Oracle {
	store := ctx.KVStore(k.priceStoreKey)
	bz := store.Get([]byte(OraclePrefix))
	var oracles []Oracle
	k.cdc.MustUnmarshalBinaryBare(bz, &oracles)
	return oracles
}

// GetAssets returns the assets in the pricefeed store
func (k Keeper) GetAssets(ctx sdk.Context) []Asset {
	store := ctx.KVStore(k.priceStoreKey)
	bz := store.Get([]byte(AssetPrefix))
	var assets []Asset
	k.cdc.MustUnmarshalBinaryBare(bz, &assets)
	return assets
}

// GetAsset returns the asset if it is in the pricefeed system
func (k Keeper) GetAsset(ctx sdk.Context, assetCode string) (Asset, bool) {
	assets := k.GetAssets(ctx)

	for i := range assets {
		if assets[i].AssetCode == assetCode {
			return assets[i], true
		}
	}
	return Asset{}, false

}

// GetOracle returns the oracle address as a string if it is in the pricefeed store
func (k Keeper) GetOracle(ctx sdk.Context, oracle string) (Oracle, bool) {
	oracles := k.GetOracles(ctx)

	for i := range oracles {
		if oracles[i].OracleAddress == oracle {
			return oracles[i], true
		}
	}
	return Oracle{}, false

}

// Deve essere estratto l'oracolo preposto a valutare quel tipo di NFT
// poi deve essere registrato un msg con l'indicazione del tipo di NFT,
// il suo ID, l'oracolo preposto e il fatto che sia o meno stato valutato
// Questo elemento del keystore serve per restitutire un messaggio agli oracoli perché valutino l'NFT
func (k Keeper) SetOracleMsg(ctx sdk.Context, token client.Token) CurrentPrice {
	/*
		1) Dal token estraggo il tipo di NFT
		2) Ricerco tra gli oracoli quello preposto per il tipo di NFT: ottengo l'address
		3) Registro un elemento in EstimableAsset (da cambiare la struttura) con Estimed che indica che non è stato stimato
		4) Successivamente EstimableAsset deve essere interrogato passando l'address dell'oracolo che saprà quali sono gli assets
		da valutare.
		5) L'oracolo fa la stima del prezzo e inserisce tramite una transazione che inserisce la stima del prezzo.
		6) Viene settato il prezzo dell'NFT
		7) Viene portato a true Estimed del EstimableAsset del punto 3



	*/

	/*
		var oracles []Oracle
		esitimableAssets := k.GetEstimableAssets(ctx)
		esitimableAssets = append(esitimableAssets, EstimableAsset{OracleAddress: oracleAddr, AssetCode: storedPriceKey, Estimed: false})
		store := ctx.KVStore(k.priceStoreKey)
		store.Set(
			[]byte(EstimableAssetPrefix), k.cdc.MustMarshalBinaryBare(esitimableAssets),
		)*/
}

// GetCurrentPrice fetches the current median price of all oracles for a specific asset
func (k Keeper) GetCurrentPrice(ctx sdk.Context, token client.Token) CurrentPrice {

	store := ctx.KVStore(k.priceStoreKey)
	var storedPriceKey string

	switch token := token.(type) {
	case cdp.BaseNFT:
		storedPriceKey = CurrentPricePrefix + token.GetName() + "++" + token.GetID()
	case cdp.BaseFT:
		storedPriceKey = CurrentPricePrefix + token.GetName()
	default:
		panic(errors.New("Unrecognized cdp token type"))
	}

	bz := store.Get([]byte(storedPriceKey))

	var price CurrentPrice
	k.cdc.MustUnmarshalBinaryBare(bz, &price)

	if price.Price.IsZero() {
		if token.TokenType() == _NFT {
			k.SetOracleMsg(ctx, token)
		}
	}

	return price
}

// GetRawPrices fetches the set of all prices posted by oracles for an asset
func (k Keeper) GetRawPrices(ctx sdk.Context, assetCode string) []PostedPrice {
	store := ctx.KVStore(k.priceStoreKey)
	bz := store.Get([]byte(RawPriceFeedPrefix + assetCode))
	var prices []PostedPrice
	k.cdc.MustUnmarshalBinaryBare(bz, &prices)
	return prices
}

// ValidatePostPrice makes sure the person posting the price is an oracle
func (k Keeper) ValidatePostPrice(ctx sdk.Context, msg MsgPostPrice) sdk.Error {
	// TODO implement this

	_, assetFound := k.GetAsset(ctx, msg.AssetCode)
	if !assetFound {
		return ErrInvalidAsset(k.codespace)
	}
	_, oracleFound := k.GetOracle(ctx, msg.From.String())
	if !oracleFound {
		return ErrInvalidOracle(k.codespace)
	}

	return nil
}
