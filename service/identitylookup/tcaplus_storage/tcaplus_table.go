package tcaplus_storage

import (
	"fmt"
	"time"

	"gitee.com/wxdqing/identitylookup/errorx"
	"github.com/tencentyun/tcaplusdb-go-sdk/pb/terror"
	"google.golang.org/protobuf/proto"

	cli "github.com/tencentyun/tcaplusdb-go-sdk/pb"
	"github.com/tencentyun/tcaplusdb-go-sdk/pb/protocol/cmd"
	"github.com/tencentyun/tcaplusdb-go-sdk/pb/protocol/tcaplus_protocol_cs"
	"github.com/tencentyun/tcaplusdb-go-sdk/pb/response"
)

type TcaplusTable struct {
	client      *cli.PBClient
	zoneID      uint32
	tableName   string
	messageType proto.Message
	keyFields   []string
	timeout     time.Duration
}

func NewTcaplusTable(client *cli.PBClient, zoneID uint32, tableName string, messageType proto.Message, keyFields []string) *TcaplusTable {
	return &TcaplusTable{
		client:      client,
		zoneID:      zoneID,
		tableName:   tableName,
		messageType: messageType,
		keyFields:   keyFields,
		timeout:     time.Second * 5,
	}
}

func (c *TcaplusTable) getDataWithVersion(resp response.TcaplusResponse, msg proto.Message) (proto.Message, int32, error) {
	// 检查响应结果。
	result := resp.GetResult()
	if result != terror.GEN_ERR_SUC {
		if result == terror.TXHDB_ERR_RECORD_NOT_EXIST {
			return nil, 0, nil // 记录不存在。
		}

		return nil, 0, fmt.Errorf("error getting record: %d", result)
	}
	// 获取记录。
	record, err := resp.FetchRecord()
	if err != nil {
		return nil, 0, fmt.Errorf("error fetching record: %w", err)
	}

	// 获取记录版本和数据。
	version := record.GetVersion()
	if err := record.GetPBData(msg); err != nil {
		return nil, 0, fmt.Errorf("error getting data: %w", err)
	}

	return msg, version, nil
}

// GetRecord 获取记录，并返回记录数据和版本。
func (c *TcaplusTable) GetRecord(keyValues []string) (proto.Message, int32, error) {
	req, err := c.client.NewRequest(c.zoneID, c.tableName, cmd.TcaplusApiGetReq)
	if err != nil {
		return nil, 0, fmt.Errorf("error creating get request: %w", err)
	}

	// 复制消息模板并设置主键字段。
	msg := proto.Clone(c.messageType)
	rec, err := req.AddRecord(0)
	if err != nil {
		return nil, 0, fmt.Errorf("error adding record: %w", err)
	}

	for index, key := range c.keyFields {
		setFieldValueStringUnsafe(msg, key, keyValues[index])
	}

	_, err = rec.SetPBData(msg)
	if err != nil {
		return nil, 0, fmt.Errorf("error setting pb data: %w", err)
	}

	// 发送请求。
	resp, err := c.client.Do(req, c.timeout)
	if err != nil {
		return nil, 0, fmt.Errorf("error sending get request: %w", err)
	}

	// 获取记录版本和数据。
	return c.getDataWithVersion(resp, msg)
}

// InsertRecord 插入记录，并返回写入后的记录数据和版本。
func (c *TcaplusTable) InsertRecord(keyValues []string, record proto.Message) (proto.Message, int32, error) {
	req, err := c.client.NewRequest(c.zoneID, c.tableName, cmd.TcaplusApiInsertReq)
	if err != nil {
		return nil, 0, fmt.Errorf("error creating insert request: %w", err)
	}
	req.SetResultFlagForSuccess(byte(tcaplus_protocol_cs.TCaplusValueFlag_ALLVALUE))

	// 复制记录并设置主键字段。
	msg := proto.Clone(record)
	rec, err := req.AddRecord(0)
	if err != nil {
		return nil, 0, fmt.Errorf("error adding record: %w", err)
	}

	for index, key := range c.keyFields {
		setFieldValueStringUnsafe(msg, key, keyValues[index])
	}

	_, err = rec.SetPBData(msg)
	if err != nil {
		return nil, 0, fmt.Errorf("error setting pb data: %w", err)
	}

	// 发送请求。
	resp, err := c.client.Do(req, c.timeout)
	if err != nil {
		return nil, 0, fmt.Errorf("error sending insert request: %w", err)
	}

	result := resp.GetResult()
	if result != terror.GEN_ERR_SUC {
		if result == terror.SVR_ERR_FAIL_RECORD_EXIST {
			return nil, 0, errorx.ErrDBRecordExist
		}

		return nil, 0, fmt.Errorf("error getting record: %d", result)
	}

	return c.getDataWithVersion(resp, msg)
}

// UpdateRecord 按预期版本更新记录，并返回更新后的记录数据和版本。
func (c *TcaplusTable) UpdateRecord(keyValues []string, record proto.Message, expectedVersion int32) (proto.Message, int32, error) {
	req, err := c.client.NewRequest(c.zoneID, c.tableName, cmd.TcaplusApiUpdateReq)
	if err != nil {
		return nil, 0, fmt.Errorf("error creating update request: %w", err)
	}
	req.SetResultFlagForSuccess(byte(tcaplus_protocol_cs.TCaplusValueFlag_ALLVALUE))

	// 复制记录并设置主键字段。
	msg := proto.Clone(record)
	rec, err := req.AddRecord(0)
	if err != nil {
		return nil, 0, fmt.Errorf("error adding record: %w", err)
	}

	for index, key := range c.keyFields {
		setFieldValueStringUnsafe(msg, key, keyValues[index])
	}

	_, err = rec.SetPBData(msg)
	if err != nil {
		return nil, 0, fmt.Errorf("error setting pb data: %w", err)
	}

	// 设置期望版本，用于 CAS 更新。
	if expectedVersion <= 0 {
		return nil, 0, fmt.Errorf("error setting version:%d %w", expectedVersion, err)
	}
	rec.SetVersion(expectedVersion)

	resp, err := c.client.Do(req, c.timeout)
	if err != nil {
		return nil, 0, fmt.Errorf("error sending update request: %w", err)
	}

	result := resp.GetResult()
	if result != terror.GEN_ERR_SUC {
		if result == terror.SVR_ERR_FAIL_INVALID_VERSION {
			return nil, 0, errorx.ErrDBInvalidVersion
		}

		return nil, 0, fmt.Errorf("error getting record: %d", result)
	}

	return c.getDataWithVersion(resp, msg)
}

// DeleteRecord 按预期版本删除记录。
func (c *TcaplusTable) DeleteRecord(keyValues []string, expectedVersion int32) error {
	req, err := c.client.NewRequest(c.zoneID, c.tableName, cmd.TcaplusApiDeleteReq)
	if err != nil {
		return fmt.Errorf("error creating delete request: %w", err)
	}

	// 复制消息模板并设置主键字段。
	msg := proto.Clone(c.messageType)
	rec, err := req.AddRecord(0)
	if err != nil {
		return fmt.Errorf("error adding record: %w", err)
	}

	for index, key := range c.keyFields {
		setFieldValueStringUnsafe(msg, key, keyValues[index])
	}

	_, err = rec.SetPBData(msg)
	if err != nil {
		return fmt.Errorf("error setting pb data: %w", err)
	}

	// 设置期望版本，用于 CAS 删除。
	if expectedVersion > 0 {
		rec.SetVersion(expectedVersion)
	}

	// 发送请求。
	resp, err := c.client.Do(req, c.timeout)
	if err != nil {
		return fmt.Errorf("error sending delete request: %w", err)
	}

	// 检查响应结果。
	if resp.GetResult() != terror.GEN_ERR_SUC {
		return fmt.Errorf("delete failed with result: %d", resp.GetResult())
	}

	return nil
}
