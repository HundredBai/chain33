// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package addrindex

import (
	"fmt"

	"github.com/33cn/chain33/common/address"
	dbm "github.com/33cn/chain33/common/db"
	log "github.com/33cn/chain33/common/log/log15"
	drivers "github.com/33cn/chain33/system/dapp"
	"github.com/33cn/chain33/system/plugin"
	"github.com/33cn/chain33/types"
)

var (
	name = "addrindex"
	elog = log.New("module", "system/plugin/txindex")
)

func init() {
	plugin.RegisterPlugin(name, newAddrindex())
	plugin.RegisterQuery("GetTxsByAddr", name)
}

type addrindexPlugin struct {
	*plugin.Base
}

func newAddrindex() *addrindexPlugin {
	return &addrindexPlugin{Base: &plugin.Base{}}
}

func (p *addrindexPlugin) CheckEnable(enable bool) (kvs []*types.KeyValue, ok bool, err error) {
	return nil, enable, nil
}

func (p *addrindexPlugin) ExecLocal(data *types.BlockDetail) ([]*types.KeyValue, error) {
	b := data.Block
	var set types.LocalDBSet
	for i := 0; i < len(b.Txs); i++ {
		tx := b.Txs[i]
		receipt := data.Receipts[i]
		txindex := getTxIndex(p, tx, receipt, i)
		txinfobyte := types.Encode(txindex.index)
		if len(txindex.from) != 0 {
			fromkey1 := CalcTxAddrDirHashKey(name, txindex.from, drivers.TxIndexFrom, txindex.heightstr)
			fromkey2 := CalcTxAddrHashKey(name, txindex.from, txindex.heightstr)
			set.KV = append(set.KV, &types.KeyValue{Key: fromkey1, Value: txinfobyte})
			set.KV = append(set.KV, &types.KeyValue{Key: fromkey2, Value: txinfobyte})
			types.AssertConfig(p.GetAPI())
			kv, err := updateAddrTxsCount(p.GetAPI().GetConfig(), p.GetLocalDB(), txindex.from, 1, true)
			if err == nil && kv != nil {
				set.KV = append(set.KV, kv)
			}
		}
		if len(txindex.to) != 0 {
			tokey1 := CalcTxAddrDirHashKey(name, txindex.to, drivers.TxIndexTo, txindex.heightstr)
			tokey2 := CalcTxAddrHashKey(name, txindex.to, txindex.heightstr)
			set.KV = append(set.KV, &types.KeyValue{Key: tokey1, Value: txinfobyte})
			set.KV = append(set.KV, &types.KeyValue{Key: tokey2, Value: txinfobyte})
			types.AssertConfig(p.GetAPI())
			kv, err := updateAddrTxsCount(p.GetAPI().GetConfig(), p.GetLocalDB(), txindex.to, 1, true)
			if err == nil && kv != nil {
				set.KV = append(set.KV, kv)
			}
		}
	}
	return set.KV, nil
}

func (p *addrindexPlugin) ExecDelLocal(data *types.BlockDetail) ([]*types.KeyValue, error) {
	b := data.Block
	var set types.LocalDBSet
	for i := 0; i < len(b.Txs); i++ {
		tx := b.Txs[i]
		receipt := data.Receipts[i]
		//del: addr index
		txindex := getTxIndex(p, tx, receipt, i)
		if len(txindex.from) != 0 {
			fromkey1 := CalcTxAddrDirHashKey(name, txindex.from, drivers.TxIndexFrom, txindex.heightstr)
			fromkey2 := CalcTxAddrHashKey(name, txindex.from, txindex.heightstr)
			set.KV = append(set.KV, &types.KeyValue{Key: fromkey1, Value: nil})
			set.KV = append(set.KV, &types.KeyValue{Key: fromkey2, Value: nil})
			kv, err := updateAddrTxsCount(p.GetAPI().GetConfig(), p.GetLocalDB(), txindex.from, 1, false)
			if err == nil && kv != nil {
				set.KV = append(set.KV, kv)
			}
		}
		if len(txindex.to) != 0 {
			tokey1 := CalcTxAddrDirHashKey(name, txindex.to, drivers.TxIndexTo, txindex.heightstr)
			tokey2 := CalcTxAddrHashKey(name, txindex.to, txindex.heightstr)
			set.KV = append(set.KV, &types.KeyValue{Key: tokey1, Value: nil})
			set.KV = append(set.KV, &types.KeyValue{Key: tokey2, Value: nil})
			kv, err := updateAddrTxsCount(p.GetAPI().GetConfig(), p.GetLocalDB(), txindex.to, 1, false)
			if err == nil && kv != nil {
				set.KV = append(set.KV, kv)
			}
		}
	}
	return set.KV, nil
}

func getAddrTxsCountKV(addr string, count int64) *types.KeyValue {
	counts := &types.Int64{Data: count}
	countbytes := types.Encode(counts)
	kv := &types.KeyValue{Key: CalcAddrTxsCountKey(name, addr), Value: countbytes}
	return kv
}

func getAddrTxsCount(db dbm.KVDB, addr string) (int64, error) {
	count := types.Int64{}
	TxsCount, err := db.Get(CalcAddrTxsCountKey(name, addr))
	if err != nil && err != types.ErrNotFound {
		return 0, err
	}
	if len(TxsCount) == 0 {
		return 0, nil
	}
	err = types.Decode(TxsCount, &count)
	if err != nil {
		return 0, err
	}
	return count.Data, nil
}

func setAddrTxsCount(db dbm.KVDB, addr string, count int64) error {
	kv := getAddrTxsCountKV(addr, count)
	return db.Set(kv.Key, kv.Value)
}

func updateAddrTxsCount(cfg *types.Chain33Config, cachedb dbm.KVDB, addr string, amount int64, isadd bool) (*types.KeyValue, error) {
	//blockchaindb 数据库0版本不支持此功能
	ver := cfg.GInt("dbversion")
	if ver == 0 {
		return nil, types.ErrNotFound
	}
	txscount, err := getAddrTxsCount(cachedb, addr)
	if err != nil && err != types.ErrNotFound {
		return nil, err
	}
	if isadd {
		txscount += amount
	} else {
		txscount -= amount
	}
	err = setAddrTxsCount(cachedb, addr, txscount)
	if err != nil {
		return nil, err
	}
	//keyvalue
	return getAddrTxsCountKV(addr, txscount), nil
}

type txIndex struct {
	from      string
	to        string
	heightstr string
	index     *types.ReplyTxInfo
}

//交易中 from/to 的索引
func getTxIndex(p plugin.Plugin, tx *types.Transaction, receipt *types.ReceiptData, index int) *txIndex {
	var txIndexInfo txIndex
	var txinf types.ReplyTxInfo
	txinf.Hash = tx.Hash()
	txinf.Height = p.GetHeight()
	txinf.Index = int64(index)
	ety := types.LoadExecutorType(string(tx.Execer))
	// none exec has not execType
	if ety != nil {
		var err error
		txinf.Assets, err = ety.GetAssets(tx)
		if err != nil {
			elog.Error("getTxIndex ", "GetAssets err", err)
		}
	}

	txIndexInfo.index = &txinf
	heightstr := fmt.Sprintf("%018d", p.GetHeight()*types.MaxTxsPerBlock+int64(index))
	txIndexInfo.heightstr = heightstr

	txIndexInfo.from = address.PubKeyToAddress(tx.GetSignature().GetPubkey()).String()
	txIndexInfo.to = tx.GetRealToAddr()
	return &txIndexInfo
}

// local keys
var (
	TxAddrHash    = []byte("H")  // TxAddrHash
	TxAddrDirHash = []byte("DH") // TxAddrDirHash
	AddrTxsCount  = []byte("C")  // AddrTxsCount
)

//CalcTxAddrHashKey 用于存储地址相关的hash列表，key=TxAddrHash:addr:height*100000 + index
//地址下面所有的交易
func CalcTxAddrHashKey(name, addr string, heightindex string) []byte {
	return []byte(fmt.Sprintf("%s-%s-%s:%s:%s", types.LocalPluginPrefix, name, TxAddrHash, addr, heightindex))
}

//CalcTxAddrDirHashKey 用于存储地址相关的hash列表，key=TxAddrHash:addr:flag:height*100000 + index
//地址下面某个分类的交易
func CalcTxAddrDirHashKey(name, addr string, flag int32, heightindex string) []byte {
	return []byte(fmt.Sprintf("%s-%s-%s:%s:%d:%s", types.LocalPluginPrefix, name, TxAddrDirHash, addr, flag, heightindex))
}

//CalcAddrTxsCountKey 存储地址参与的交易数量。add时加一，del时减一
func CalcAddrTxsCountKey(name, addr string) []byte {
	return []byte(fmt.Sprintf("%s-%s-%s:%s", types.LocalPluginPrefix, name, AddrTxsCount, addr))
}
