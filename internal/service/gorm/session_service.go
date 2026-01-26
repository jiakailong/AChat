package gorm

import (
	"encoding/json"
	"errors"
	"fmt"
	"kama_chat_server/internal/dao"
	"kama_chat_server/internal/dto/request"
	"kama_chat_server/internal/dto/respond"
	"kama_chat_server/internal/model"
	myredis "kama_chat_server/internal/service/redis"
	"kama_chat_server/pkg/constants"
	"kama_chat_server/pkg/enum/contact/contact_status_enum"
	"kama_chat_server/pkg/enum/group_info/group_status_enum"
	"kama_chat_server/pkg/enum/user_info/user_status_enum"
	"kama_chat_server/pkg/util/random"
	"kama_chat_server/pkg/zlog"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type sessionService struct {
	sessionDAO dao.SessionDAO
	userDAO    dao.UserDAO
	groupDAO   dao.GroupDAO
}

var SessionService *sessionService

func InitSessionService(sessionDao dao.SessionDAO, userDao dao.UserDAO, groupDao dao.GroupDAO) {
	SessionService = &sessionService{
		sessionDAO: sessionDao,
		userDAO:    userDao,
		groupDAO:   groupDao,
	}
}

// CreateSession 创建会话
func (s *sessionService) CreateSession(req request.CreateSessionRequest) (string, string, int) {
	_, err := s.userDAO.GetUserByUUID(req.SendId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, "", -1
	}
	var session model.Session
	session.Uuid = fmt.Sprintf("S%s", random.GetNowAndLenRandomString(11))
	session.SendId = req.SendId
	session.ReceiveId = req.ReceiveId
	session.CreatedAt = time.Now()
	if req.ReceiveId[0] == 'U' {
		receiveUser, err := s.userDAO.GetUserByUUID(req.ReceiveId)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, "", -1
		}
		if receiveUser.Status == user_status_enum.DISABLE {
			zlog.Error("该用户被禁用了")
			return "该用户被禁用了", "", -2
		} else {
			session.ReceiveName = receiveUser.Nickname
			session.Avatar = receiveUser.Avatar
		}
	} else {
		receiveGroup, err := s.groupDAO.GetGroupByUUID(req.ReceiveId)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, "", -1
		}
		if receiveGroup.Status == group_status_enum.DISABLE {
			zlog.Error("该群聊被禁用了")
			return "该群聊被禁用了", "", -2
		} else {
			session.ReceiveName = receiveGroup.Name
			session.Avatar = receiveGroup.Avatar
		}
	}

	if err := s.sessionDAO.CreateSession(&session); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, "", -1
	}
	if err := myredis.DelKeysWithPattern("group_session_list_" + req.SendId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("session_list_" + req.SendId); err != nil {
		zlog.Error(err.Error())
	}
	return "会话创建成功", session.Uuid, 0
}

// CheckOpenSessionAllowed 检查是否允许发起会话
func (s *sessionService) CheckOpenSessionAllowed(sendId, receiveId string) (string, bool, int) {
	// 1. 检查联系人关系
	contact, err := s.userDAO.GetUserContact(sendId, receiveId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, false, -1
	}
	if contact.Status == contact_status_enum.BE_BLACK {
		return "已被对方拉黑，无法发起会话", false, -2
	} else if contact.Status == contact_status_enum.BLACK {
		return "已拉黑对方，先解除拉黑状态才能发起会话", false, -2
	}

	// 2. 检查目标状态
	if receiveId[0] == 'U' {
		user, err := s.userDAO.GetUserByUUID(receiveId)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, false, -1
		}
		if user.Status == user_status_enum.DISABLE {
			zlog.Info("对方已被禁用，无法发起会话")
			return "对方已被禁用，无法发起会话", false, -2
		}
	} else {
		group, err := s.groupDAO.GetGroupByUUID(receiveId)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, false, -1
		}
		if group.Status == group_status_enum.DISABLE {
			zlog.Info("对方已被禁用，无法发起会话")
			return "对方已被禁用，无法发起会话", false, -2
		}
	}
	return "可以发起会话", true, 0
}

// DeleteSession 删除会话

// OpenSession 打开会话
func (s *sessionService) OpenSession(req request.OpenSessionRequest) (string, string, int) {
	rspString, err := myredis.GetKeyWithPrefixNilIsErr("session_" + req.SendId + "_" + req.ReceiveId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 查询 DB
			session, err := s.sessionDAO.GetSessionBySendAndReceive(req.SendId, req.ReceiveId)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					zlog.Info("会话没有找到，将新建会话")
					createReq := request.CreateSessionRequest{
						SendId:    req.SendId,
						ReceiveId: req.ReceiveId,
					}
					return s.CreateSession(createReq)
				}
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, "", -1
			}
			return "会话创建成功", session.Uuid, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, "", -1
		}
	}
	var session model.Session
	if err := json.Unmarshal([]byte(rspString), &session); err != nil {
		zlog.Error(err.Error())
	}
	return "会话创建成功", session.Uuid, 0
}

// GetUserSessionList 获取用户会话列表
func (s *sessionService) GetUserSessionList(ownerId string) (string, []respond.UserSessionListRespond, int) {
	rspString, err := myredis.GetKeyNilIsErr("session_list_" + ownerId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			var sessionList []model.Session
			if res := dao.GormDB.Order("created_at DESC").Where("send_id = ?", ownerId).Find(&sessionList); res.Error != nil {
				if errors.Is(res.Error, gorm.ErrRecordNotFound) {
					zlog.Info("未创建用户会话")
					return "未创建用户会话", nil, 0
				} else {
					zlog.Error(res.Error.Error())
					return constants.SYSTEM_ERROR, nil, -1
				}
			}
			var sessionListRsp []respond.UserSessionListRespond
			for i := 0; i < len(sessionList); i++ {
				if sessionList[i].ReceiveId[0] == 'U' {
					sessionListRsp = append(sessionListRsp, respond.UserSessionListRespond{
						SessionId: sessionList[i].Uuid,
						Avatar:    sessionList[i].Avatar,
						UserId:    sessionList[i].ReceiveId,
						Username:  sessionList[i].ReceiveName,
					})
				}
			}
			rspString, err := json.Marshal(sessionListRsp)
			if err != nil {
				zlog.Error(err.Error())
			}
			if err := myredis.SetKeyEx("session_list_"+ownerId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
				zlog.Error(err.Error())
			}
			return "获取成功", sessionListRsp, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	var rsp []respond.UserSessionListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取成功", rsp, 0
}

// GetGroupSessionList 获取群聊会话列表
func (s *sessionService) GetGroupSessionList(ownerId string) (string, []respond.GroupSessionListRespond, int) {
	rspString, err := myredis.GetKeyNilIsErr("group_session_list_" + ownerId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 直接调用 DAO
			sessionList, err := s.sessionDAO.GetGroupSessionList(ownerId)
			if err != nil {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			if len(sessionList) == 0 {
				zlog.Info("未创建群聊会话")
				return "未创建群聊会话", nil, 0
			}

			var sessionListRsp []respond.GroupSessionListRespond
			// 不需要在内存中 if sessionList[i].ReceiveId[0] == 'G'
			for _, session := range sessionList {
				sessionListRsp = append(sessionListRsp, respond.GroupSessionListRespond{
					SessionId: session.Uuid,
					Avatar:    session.Avatar,
					GroupId:   session.ReceiveId,
					GroupName: session.ReceiveName,
				})
			}
			rspString, err := json.Marshal(sessionListRsp)
			if err != nil {
				zlog.Error(err.Error())
			}
			if err := myredis.SetKeyEx("group_session_list_"+ownerId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
				zlog.Error(err.Error())
			}
			return "获取成功", sessionListRsp, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	var rsp []respond.GroupSessionListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取成功", rsp, 0
}

// DeleteSession 删除会话
func (s *sessionService) DeleteSession(ownerId, sessionId string) (string, int) {
	session, err := s.sessionDAO.GetSessionByUUID(sessionId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}

	session.DeletedAt.Valid = true
	session.DeletedAt.Time = time.Now()

	if err := s.sessionDAO.UpdateSession(session); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}

	// 缓存清理
	if err := myredis.DelKeysWithPattern("group_session_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("session_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}
	return "删除成功", 0
}
