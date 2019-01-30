// +build !windows

package base

import "C"
import (
	"github.com/gw123/GMQ/core/interfaces"
	"syscall"
	"unsafe"
	"encoding/json"
	"strconv"
	"strings"
	"github.com/gw123/GMQ/common/common_types"
	"plugin"
)

type DllModule struct {
	BaseModule
	eventNames []string
	moduleDll  *plugin.Plugin
	handel     *plugin.Symbol
	start      *plugin.Symbol
	stop       *plugin.Symbol
	getVersion *plugin.Symbol
}

func NewDllModule() *DllModule {
	return new(DllModule)
}

func (this *DllModule) GetStatus() uint64 {
	return 1
}

func (this *DllModule) Init(app interfaces.App, config interfaces.ModuleConfig) error {
	this.BaseModule.Init(app, config)
	var err error
	path := config.GetPath()
	//this.Debug("path :" + path)
	this.moduleDll, err = plugin.Open(path)

	if err != nil {
		this.Error("LoadDLL faild " + this.GetModuleName())
		return err
	}
	this.handel, err = this.moduleDll.Lookup("handle")
	if err != nil {
		this.Error("FindProc handel faild " + this.GetModuleName())
		return err
	}

	this.start, err = this.moduleDll.Lookup("start")
	if err != nil {
		this.Error("FindProc start faild " + this.GetModuleName())
		return err
	}

	this.stop, err = this.moduleDll.Lookup("stop")
	if err != nil {
		this.Error("FindProc stop faild " + this.GetModuleName())
		return err
	}
	//C.GoString((*C.char)(unsafe.Pointer(version)))
	this.getVersion, err = this.moduleDll.Lookup("getModuleVersion")
	if err != nil {
		this.Warning("FindProc getVersion faild " + this.GetModuleName())
		this.Version = ""
	} else {
		version, _, _ := this.getVersion()
		this.Version = C.GoString((*C.char)(unsafe.Pointer(version)))
	}

	var callback = func(event *C.char) uintptr {
		eventStr := C.GoString(event)
		this.Debug("Callback" + this.GetModuleName() + " " + eventStr)
		//fmt.Println("callback:", eventStr)
		resultEvent := &common_types.ResultEvent{}
		err := json.Unmarshal([]byte(eventStr), resultEvent)
		if err != nil {
			this.Error("Call back Json.Unmarshal " + err.Error())
			return 1
		}
		this.Pub(resultEvent)
		return 0
	}
	//
	items := this.Config.GetItems()
	data, err := json.Marshal(items)
	if err != nil {
		this.Error("json.Marshal(items) " + err.Error())
		return err
	}
	this.Debug("loaded path :" + string(data))
	status, _, err := this.start.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(syscall.NewCallback(callback)))
	//err.Error() != "The operation completed successfully."
	if status != 0 {
		this.Error("status" + strconv.Itoa(int(status)) + err.Error())
		return err
	}

	this.Start()
	return nil
}

func (this *DllModule) Start() {
	go func() {
		for ; ; {
			event := this.Pop()
			this.App.Debug(this.GetModuleName(), event.GetEventName()+" "+string(event.GetPayload()))
			type Event struct {
				MsgId     string
				EventName string
				Payload   string
			}
			ev := Event{
				MsgId:     event.GetMsgId(),
				EventName: event.GetEventName(),
				Payload:   string(event.GetPayload()),
			}
			jsonData, err := json.Marshal(ev)

			//fmt.Println("jsonData", jsonData)
			//e := common.LhMsg{}
			//err = json.Unmarshal(jsonData, &ev)
			//fmt.Println("err", err)
			//err = json.Unmarshal([]byte(ev.Payload), &e)
			//fmt.Println("err", err)
			//fmt.Println("decode", e)

			res, _, err := this.handel.Call(uintptr(unsafe.Pointer(&jsonData[0])))
			if res == 0 {
				if !strings.Contains(err.Error(), "successfully") {
					this.Warning(event.GetMsgId() + " " + event.GetEventName() + "系统调用error: " + err.Error())
				}
				//执行成功
				//replay := common.NewResultEvent([]byte("执行成功"))
				//this.App.Pub(replay)
				this.Info(event.GetMsgId() + " " + event.GetEventName() + " 执行成功")
			} else {
				//replay := common.NewResultEvent([]byte("执行失败"))
				//this.App.Pub(replay)
				this.Error(event.GetMsgId() + " " + event.GetEventName() + " 执行失败 " + err.Error())
			}
		}
	}()
}

func (this *DllModule) UnInit() (err error) {
	this.BaseModule.UnInit()
	status, _, err := this.stop.Call()
	if status != 0 {
		this.Error("status" + strconv.Itoa(int(status)) + err.Error())
	}
	this.moduleDll.Release()
	this.start = nil
	this.stop = nil
	this.handel = nil
	return err
}
