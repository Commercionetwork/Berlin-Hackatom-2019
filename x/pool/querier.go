package pool

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

const (
	QueryTotalFunds = "funds"
	QueryReadFunds  = "get-funds"
)

func NewQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) (res []byte, err sdk.Error) {
		switch path[0] {
		case QueryTotalFunds:
			return queryGetTotalFunds(ctx, req, keeper)
		case QueryReadFunds:
			return queryGetFunds(ctx, req, keeper)
		default:
			return nil, sdk.ErrUnknownRequest("unknown pool query endpoint")
		}
	}
}

type QueryFundsParams struct {
	Owner sdk.AccAddress
}

// queryGetFunds fetched the funds for a specific owner
func queryGetFunds(ctx sdk.Context, req abci.RequestQuery, keeper Keeper) ([]byte, sdk.Error) {
	// Decode request
	var requestParams QueryFundsParams
	err := keeper.cdc.UnmarshalJSON(req.Data, &requestParams)
	if err != nil {
		return nil, sdk.ErrInternal(fmt.Sprintf("failed to parse params: %s", err))
	}

	// Get CDPs
	fund, err := keeper.GetAccountFunds(ctx, requestParams.Owner)
	if err != nil {
		return nil, sdk.ErrInternal(err.Error())
	}

	// Encode results
	bz, err := codec.MarshalJSONIndent(keeper.cdc, fund)
	if err != nil {
		return nil, sdk.ErrInternal(sdk.AppendMsgToErr("could not marshal result to JSON", err.Error()))
	}
	return bz, nil
}

func queryGetTotalFunds(ctx sdk.Context, _ abci.RequestQuery, keeper Keeper) ([]byte, sdk.Error) {

	var bz []byte

	funds, err := keeper.GetTotalFunds(ctx)
	if err != nil {
		return bz, err
	}

	// Encode results
	bz, jsonError := codec.MarshalJSONIndent(keeper.cdc, funds)
	if jsonError != nil {
		return nil, sdk.ErrInternal(sdk.AppendMsgToErr("could not marshal result to JSON", jsonError.Error()))
	}
	return bz, nil
}
