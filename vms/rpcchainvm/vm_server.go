// Copyright (C) 2019-2021, Dijets Inc, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpcchainvm

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hashicorp/go-plugin"

	"github.com/lasthyphen/dijetsgogo/api/keystore/gkeystore"
	"github.com/lasthyphen/dijetsgogo/api/metrics"
	"github.com/lasthyphen/dijetsgogo/api/proto/appsenderproto"
	"github.com/lasthyphen/dijetsgogo/api/proto/galiasreaderproto"
	"github.com/lasthyphen/dijetsgogo/api/proto/ghttpproto"
	"github.com/lasthyphen/dijetsgogo/api/proto/gkeystoreproto"
	"github.com/lasthyphen/dijetsgogo/api/proto/gsharedmemoryproto"
	"github.com/lasthyphen/dijetsgogo/api/proto/gsubnetlookupproto"
	"github.com/lasthyphen/dijetsgogo/api/proto/messengerproto"
	"github.com/lasthyphen/dijetsgogo/api/proto/rpcdbproto"
	"github.com/lasthyphen/dijetsgogo/api/proto/vmproto"
	"github.com/lasthyphen/dijetsgogo/chains/atomic/gsharedmemory"
	"github.com/lasthyphen/dijetsgogo/database/corruptabledb"
	"github.com/lasthyphen/dijetsgogo/database/manager"
	"github.com/lasthyphen/dijetsgogo/database/rpcdb"
	"github.com/lasthyphen/dijetsgogo/ids"
	"github.com/lasthyphen/dijetsgogo/ids/galiasreader"
	"github.com/lasthyphen/dijetsgogo/snow"
	"github.com/lasthyphen/dijetsgogo/snow/choices"
	"github.com/lasthyphen/dijetsgogo/snow/engine/common"
	"github.com/lasthyphen/dijetsgogo/snow/engine/common/appsender"
	"github.com/lasthyphen/dijetsgogo/snow/engine/snowman/block"
	"github.com/lasthyphen/dijetsgogo/utils/logging"
	"github.com/lasthyphen/dijetsgogo/utils/wrappers"
	"github.com/lasthyphen/dijetsgogo/version"
	"github.com/lasthyphen/dijetsgogo/vms/rpcchainvm/ghttp"
	"github.com/lasthyphen/dijetsgogo/vms/rpcchainvm/grpcutils"
	"github.com/lasthyphen/dijetsgogo/vms/rpcchainvm/gsubnetlookup"
	"github.com/lasthyphen/dijetsgogo/vms/rpcchainvm/messenger"
)

var (
	versionParser = version.NewDefaultApplicationParser()

	_ vmproto.VMServer = &VMServer{}
)

// VMServer is a VM that is managed over RPC.
type VMServer struct {
	vmproto.UnimplementedVMServer
	vm     block.ChainVM
	broker *plugin.GRPCBroker

	serverCloser grpcutils.ServerCloser
	connCloser   wrappers.Closer

	ctx    *snow.Context
	closed chan struct{}
}

// NewServer returns a vm instance connected to a remote vm instance
func NewServer(vm block.ChainVM, broker *plugin.GRPCBroker) *VMServer {
	return &VMServer{
		vm:     vm,
		broker: broker,
	}
}

func (vm *VMServer) Initialize(_ context.Context, req *vmproto.InitializeRequest) (*vmproto.InitializeResponse, error) {
	subnetID, err := ids.ToID(req.SubnetId)
	if err != nil {
		return nil, err
	}
	chainID, err := ids.ToID(req.ChainId)
	if err != nil {
		return nil, err
	}
	nodeID, err := ids.ToShortID(req.NodeId)
	if err != nil {
		return nil, err
	}
	xChainID, err := ids.ToID(req.XChainId)
	if err != nil {
		return nil, err
	}
	djtxAssetID, err := ids.ToID(req.DjtxAssetId)
	if err != nil {
		return nil, err
	}

	// Dial each database in the request and construct the database manager
	versionedDBs := make([]*manager.VersionedDatabase, len(req.DbServers))
	versionParser := version.NewDefaultParser()
	for i, vDBReq := range req.DbServers {
		version, err := versionParser.Parse(vDBReq.Version)
		if err != nil {
			// Ignore closing errors to return the original error
			_ = vm.connCloser.Close()
			return nil, err
		}

		dbConn, err := vm.broker.Dial(vDBReq.DbServer)
		if err != nil {
			// Ignore closing errors to return the original error
			_ = vm.connCloser.Close()
			return nil, err
		}
		vm.connCloser.Add(dbConn)
		db := rpcdb.NewClient(rpcdbproto.NewDatabaseClient(dbConn))
		versionedDBs[i] = &manager.VersionedDatabase{
			Database: corruptabledb.New(db),
			Version:  version,
		}
	}
	dbManager, err := manager.NewManagerFromDBs(versionedDBs)
	if err != nil {
		// Ignore closing errors to return the original error
		_ = vm.connCloser.Close()
		return nil, err
	}

	msgConn, err := vm.broker.Dial(req.EngineServer)
	if err != nil {
		// Ignore closing errors to return the original error
		_ = vm.connCloser.Close()
		return nil, err
	}
	vm.connCloser.Add(msgConn)

	keystoreConn, err := vm.broker.Dial(req.KeystoreServer)
	if err != nil {
		// Ignore closing error to return the original error
		_ = vm.connCloser.Close()
		return nil, err
	}
	vm.connCloser.Add(keystoreConn)

	sharedMemoryConn, err := vm.broker.Dial(req.SharedMemoryServer)
	if err != nil {
		// Ignore closing error to return the original error
		_ = vm.connCloser.Close()
		return nil, err
	}
	vm.connCloser.Add(sharedMemoryConn)

	bcLookupConn, err := vm.broker.Dial(req.BcLookupServer)
	if err != nil {
		// Ignore closing error to return the original error
		_ = vm.connCloser.Close()
		return nil, err
	}
	vm.connCloser.Add(bcLookupConn)

	snLookupConn, err := vm.broker.Dial(req.SnLookupServer)
	if err != nil {
		// Ignore closing error to return the original error
		_ = vm.connCloser.Close()
		return nil, err
	}
	vm.connCloser.Add(snLookupConn)

	appSenderConn, err := vm.broker.Dial(req.AppSenderServer)
	if err != nil {
		// Ignore closing error to return the original error
		_ = vm.connCloser.Close()
		return nil, err
	}
	vm.connCloser.Add(appSenderConn)

	msgClient := messenger.NewClient(messengerproto.NewMessengerClient(msgConn))
	keystoreClient := gkeystore.NewClient(gkeystoreproto.NewKeystoreClient(keystoreConn), vm.broker)
	sharedMemoryClient := gsharedmemory.NewClient(gsharedmemoryproto.NewSharedMemoryClient(sharedMemoryConn))
	bcLookupClient := galiasreader.NewClient(galiasreaderproto.NewAliasReaderClient(bcLookupConn))
	snLookupClient := gsubnetlookup.NewClient(gsubnetlookupproto.NewSubnetLookupClient(snLookupConn))
	appSenderClient := appsender.NewClient(appsenderproto.NewAppSenderClient(appSenderConn))

	toEngine := make(chan common.Message, 1)
	vm.closed = make(chan struct{})
	go func() {
		for {
			select {
			case msg, ok := <-toEngine:
				if !ok {
					return
				}
				// Nothing to do with the error within the goroutine
				_ = msgClient.Notify(msg)
			case <-vm.closed:
				return
			}
		}
	}()

	vm.ctx = &snow.Context{
		NetworkID: req.NetworkId,
		SubnetID:  subnetID,
		ChainID:   chainID,
		NodeID:    nodeID,

		XChainID:    xChainID,
		DJTXAssetID: djtxAssetID,

		Log:          logging.NoLog{},
		Keystore:     keystoreClient,
		SharedMemory: sharedMemoryClient,
		BCLookup:     bcLookupClient,
		SNLookup:     snLookupClient,
		Metrics:      metrics.NewOptionalGatherer(),

		// TODO: support snowman++ fields
	}

	if err := vm.vm.Initialize(vm.ctx, dbManager, req.GenesisBytes, req.UpgradeBytes, req.ConfigBytes, toEngine, nil, appSenderClient); err != nil {
		// Ignore errors closing resources to return the original error
		_ = vm.connCloser.Close()
		close(vm.closed)
		return nil, err
	}

	lastAccepted, err := vm.vm.LastAccepted()
	if err != nil {
		// Ignore errors closing resources to return the original error
		_ = vm.vm.Shutdown()
		_ = vm.connCloser.Close()
		close(vm.closed)
		return nil, err
	}

	blk, err := vm.vm.GetBlock(lastAccepted)
	if err != nil {
		// Ignore errors closing resources to return the original error
		_ = vm.vm.Shutdown()
		_ = vm.connCloser.Close()
		close(vm.closed)
		return nil, err
	}
	parentID := blk.Parent()
	timeBytes, err := blk.Timestamp().MarshalBinary()
	return &vmproto.InitializeResponse{
		LastAcceptedId:       lastAccepted[:],
		LastAcceptedParentId: parentID[:],
		Status:               uint32(choices.Accepted),
		Height:               blk.Height(),
		Bytes:                blk.Bytes(),
		Timestamp:            timeBytes,
	}, err
}

func (vm *VMServer) VerifyHeightIndex(context.Context, *emptypb.Empty) (*vmproto.VerifyHeightIndexResponse, error) {
	var err error
	if hVM, ok := vm.vm.(block.HeightIndexedChainVM); ok {
		err = hVM.VerifyHeightIndex()
	} else {
		err = block.ErrHeightIndexedVMNotImplemented
	}
	return &vmproto.VerifyHeightIndexResponse{
		Err: errorToErrCode[err],
	}, errorToRPCError(err)
}

func (vm *VMServer) GetBlockIDAtHeight(ctx context.Context, req *vmproto.GetBlockIDAtHeightRequest) (*vmproto.GetBlockIDAtHeightResponse, error) {
	var (
		blkID ids.ID
		err   error
	)
	if hVM, ok := vm.vm.(block.HeightIndexedChainVM); ok {
		blkID, err = hVM.GetBlockIDAtHeight(req.Height)
	} else {
		err = block.ErrHeightIndexedVMNotImplemented
	}
	return &vmproto.GetBlockIDAtHeightResponse{
		BlkId: blkID[:],
		Err:   errorToErrCode[err],
	}, errorToRPCError(err)
}

func (vm *VMServer) SetState(_ context.Context, stateReq *vmproto.SetStateRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, vm.vm.SetState(snow.State(stateReq.State))
}

func (vm *VMServer) Shutdown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	if vm.closed == nil {
		return &emptypb.Empty{}, nil
	}
	errs := wrappers.Errs{}
	errs.Add(vm.vm.Shutdown())
	close(vm.closed)
	vm.serverCloser.Stop()
	errs.Add(vm.connCloser.Close())
	return &emptypb.Empty{}, errs.Err
}

func (vm *VMServer) CreateStaticHandlers(context.Context, *emptypb.Empty) (*vmproto.CreateStaticHandlersResponse, error) {
	handlers, err := vm.vm.CreateStaticHandlers()
	if err != nil {
		return nil, err
	}
	resp := &vmproto.CreateStaticHandlersResponse{}
	for prefix, h := range handlers {
		handler := h

		// start the messenger server
		serverID := vm.broker.NextId()
		go vm.broker.AcceptAndServe(serverID, func(opts []grpc.ServerOption) *grpc.Server {
			opts = append(opts, serverOptions...)
			server := grpc.NewServer(opts...)
			vm.serverCloser.Add(server)
			ghttpproto.RegisterHTTPServer(server, ghttp.NewServer(handler.Handler, vm.broker))
			return server
		})

		resp.Handlers = append(resp.Handlers, &vmproto.Handler{
			Prefix:      prefix,
			LockOptions: uint32(handler.LockOptions),
			Server:      serverID,
		})
	}
	return resp, nil
}

func (vm *VMServer) CreateHandlers(context.Context, *emptypb.Empty) (*vmproto.CreateHandlersResponse, error) {
	handlers, err := vm.vm.CreateHandlers()
	if err != nil {
		return nil, err
	}
	resp := &vmproto.CreateHandlersResponse{}
	for prefix, h := range handlers {
		handler := h

		// start the messenger server
		serverID := vm.broker.NextId()
		go vm.broker.AcceptAndServe(serverID, func(opts []grpc.ServerOption) *grpc.Server {
			opts = append(opts, serverOptions...)
			server := grpc.NewServer(opts...)
			vm.serverCloser.Add(server)
			ghttpproto.RegisterHTTPServer(server, ghttp.NewServer(handler.Handler, vm.broker))
			return server
		})

		resp.Handlers = append(resp.Handlers, &vmproto.Handler{
			Prefix:      prefix,
			LockOptions: uint32(handler.LockOptions),
			Server:      serverID,
		})
	}
	return resp, nil
}

func (vm *VMServer) BuildBlock(context.Context, *emptypb.Empty) (*vmproto.BuildBlockResponse, error) {
	blk, err := vm.vm.BuildBlock()
	if err != nil {
		return nil, err
	}
	blkID := blk.ID()
	parentID := blk.Parent()
	timeBytes, err := blk.Timestamp().MarshalBinary()
	return &vmproto.BuildBlockResponse{
		Id:        blkID[:],
		ParentId:  parentID[:],
		Bytes:     blk.Bytes(),
		Height:    blk.Height(),
		Timestamp: timeBytes,
	}, err
}

func (vm *VMServer) ParseBlock(_ context.Context, req *vmproto.ParseBlockRequest) (*vmproto.ParseBlockResponse, error) {
	blk, err := vm.vm.ParseBlock(req.Bytes)
	if err != nil {
		return nil, err
	}
	blkID := blk.ID()
	parentID := blk.Parent()
	timeBytes, err := blk.Timestamp().MarshalBinary()
	return &vmproto.ParseBlockResponse{
		Id:        blkID[:],
		ParentId:  parentID[:],
		Status:    uint32(blk.Status()),
		Height:    blk.Height(),
		Timestamp: timeBytes,
	}, err
}

func (vm *VMServer) GetAncestors(_ context.Context, req *vmproto.GetAncestorsRequest) (*vmproto.GetAncestorsResponse, error) {
	blkID, err := ids.ToID(req.BlkId)
	if err != nil {
		return nil, err
	}
	maxBlksNum := int(req.MaxBlocksNum)
	maxBlksSize := int(req.MaxBlocksSize)
	maxBlocksRetrivalTime := time.Duration(req.MaxBlocksRetrivalTime)

	blocks, err := block.GetAncestors(
		vm.vm,
		blkID,
		maxBlksNum,
		maxBlksSize,
		maxBlocksRetrivalTime,
	)
	return &vmproto.GetAncestorsResponse{
		BlksBytes: blocks,
	}, err
}

func (vm *VMServer) BatchedParseBlock(
	ctx context.Context,
	req *vmproto.BatchedParseBlockRequest,
) (*vmproto.BatchedParseBlockResponse, error) {
	blocks := make([]*vmproto.ParseBlockResponse, len(req.Request))
	for i, blockBytes := range req.Request {
		block, err := vm.ParseBlock(ctx, &vmproto.ParseBlockRequest{
			Bytes: blockBytes,
		})
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}
	return &vmproto.BatchedParseBlockResponse{
		Response: blocks,
	}, nil
}

func (vm *VMServer) GetBlock(_ context.Context, req *vmproto.GetBlockRequest) (*vmproto.GetBlockResponse, error) {
	id, err := ids.ToID(req.Id)
	if err != nil {
		return nil, err
	}
	blk, err := vm.vm.GetBlock(id)
	if err != nil {
		return nil, err
	}
	parentID := blk.Parent()
	timeBytes, err := blk.Timestamp().MarshalBinary()
	return &vmproto.GetBlockResponse{
		ParentId:  parentID[:],
		Bytes:     blk.Bytes(),
		Status:    uint32(blk.Status()),
		Height:    blk.Height(),
		Timestamp: timeBytes,
	}, err
}

func (vm *VMServer) SetPreference(_ context.Context, req *vmproto.SetPreferenceRequest) (*emptypb.Empty, error) {
	id, err := ids.ToID(req.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, vm.vm.SetPreference(id)
}

func (vm *VMServer) Health(context.Context, *emptypb.Empty) (*vmproto.HealthResponse, error) {
	details, err := vm.vm.HealthCheck()
	if err != nil {
		return &vmproto.HealthResponse{}, err
	}

	// Try to stringify the details
	detailsStr := "couldn't parse health check details to string"
	switch details := details.(type) {
	case nil:
		detailsStr = ""
	case string:
		detailsStr = details
	case map[string]string:
		asJSON, err := json.Marshal(details)
		if err != nil {
			detailsStr = string(asJSON)
		}
	case []byte:
		detailsStr = string(details)
	}

	return &vmproto.HealthResponse{
		Details: detailsStr,
	}, nil
}

func (vm *VMServer) Version(context.Context, *emptypb.Empty) (*vmproto.VersionResponse, error) {
	version, err := vm.vm.Version()
	return &vmproto.VersionResponse{
		Version: version,
	}, err
}

func (vm *VMServer) Connected(_ context.Context, req *vmproto.ConnectedRequest) (*emptypb.Empty, error) {
	nodeID, err := ids.ToShortID(req.NodeId)
	if err != nil {
		return nil, err
	}

	peerVersion, err := versionParser.Parse(req.Version)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, vm.vm.Connected(nodeID, peerVersion)
}

func (vm *VMServer) Disconnected(_ context.Context, req *vmproto.DisconnectedRequest) (*emptypb.Empty, error) {
	nodeID, err := ids.ToShortID(req.NodeId)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, vm.vm.Disconnected(nodeID)
}

func (vm *VMServer) AppRequest(_ context.Context, req *vmproto.AppRequestMsg) (*emptypb.Empty, error) {
	nodeID, err := ids.ToShortID(req.NodeId)
	if err != nil {
		return nil, err
	}
	var deadline time.Time
	if err := deadline.UnmarshalBinary(req.Deadline); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, vm.vm.AppRequest(nodeID, req.RequestId, deadline, req.Request)
}

func (vm *VMServer) AppRequestFailed(_ context.Context, req *vmproto.AppRequestFailedMsg) (*emptypb.Empty, error) {
	nodeID, err := ids.ToShortID(req.NodeId)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, vm.vm.AppRequestFailed(nodeID, req.RequestId)
}

func (vm *VMServer) AppResponse(_ context.Context, req *vmproto.AppResponseMsg) (*emptypb.Empty, error) {
	nodeID, err := ids.ToShortID(req.NodeId)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, vm.vm.AppResponse(nodeID, req.RequestId, req.Response)
}

func (vm *VMServer) AppGossip(_ context.Context, req *vmproto.AppGossipMsg) (*emptypb.Empty, error) {
	nodeID, err := ids.ToShortID(req.NodeId)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, vm.vm.AppGossip(nodeID, req.Msg)
}

func (vm *VMServer) BlockVerify(_ context.Context, req *vmproto.BlockVerifyRequest) (*vmproto.BlockVerifyResponse, error) {
	blk, err := vm.vm.ParseBlock(req.Bytes)
	if err != nil {
		return nil, err
	}
	if err := blk.Verify(); err != nil {
		return nil, err
	}
	timeBytes, err := blk.Timestamp().MarshalBinary()
	return &vmproto.BlockVerifyResponse{
		Timestamp: timeBytes,
	}, err
}

func (vm *VMServer) BlockAccept(_ context.Context, req *vmproto.BlockAcceptRequest) (*emptypb.Empty, error) {
	id, err := ids.ToID(req.Id)
	if err != nil {
		return nil, err
	}
	blk, err := vm.vm.GetBlock(id)
	if err != nil {
		return nil, err
	}
	if err := blk.Accept(); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (vm *VMServer) BlockReject(_ context.Context, req *vmproto.BlockRejectRequest) (*emptypb.Empty, error) {
	id, err := ids.ToID(req.Id)
	if err != nil {
		return nil, err
	}
	blk, err := vm.vm.GetBlock(id)
	if err != nil {
		return nil, err
	}
	if err := blk.Reject(); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
