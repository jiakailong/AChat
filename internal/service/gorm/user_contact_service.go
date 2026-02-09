package gorm

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"kama_chat_server/internal/dao"
	"kama_chat_server/internal/dto/request"
	"kama_chat_server/internal/dto/respond"
	"kama_chat_server/internal/model"
	myredis "kama_chat_server/internal/service/redis"
	"kama_chat_server/pkg/constants"
	"kama_chat_server/pkg/enum/contact/contact_status_enum"
	"kama_chat_server/pkg/enum/contact/contact_type_enum"
	"kama_chat_server/pkg/enum/contact_apply/contact_apply_status_enum"
	"kama_chat_server/pkg/enum/group_info/group_status_enum"
	"kama_chat_server/pkg/enum/user_info/user_status_enum"
	"kama_chat_server/pkg/util/random"
	"kama_chat_server/pkg/zlog"
)

type userContactService struct {
	userContactDao dao.UserContactDAO
	userDao        dao.UserDAO
	groupDao       dao.GroupDAO
}

var UserContactService *userContactService

func InitUserContactService(userContactDao dao.UserContactDAO, userDao dao.UserDAO, groupDao dao.GroupDAO) {
	UserContactService = &userContactService{
		userContactDao: userContactDao,
		userDao:        userDao,
		groupDao:       groupDao,
	}
}

// GetUserList 获取用户列表
// 关于用户被禁用的问题，这里查到的是所有联系人，如果被禁用或被拉黑会以弹窗的形式提醒，无法打开会话框；如果被删除，是搜索不到该联系人的。
func (u *userContactService) GetUserList(ownerId string) (string, []respond.MyUserListRespond, int) {
	rspString, err := myredis.GetKeyNilIsErr("contact_user_list_" + ownerId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			contactList, err := u.userContactDao.GetUserContacts(ownerId)
			if err != nil {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			if len(contactList) == 0 {
				message := "目前不存在联系人"
				zlog.Info(message)
				return message, nil, 0
			}
			var userListRsp []respond.MyUserListRespond
			for _, contact := range contactList {
				if contact.ContactType == contact_type_enum.USER {
					user, err := u.userDao.GetUserByUUID(contact.ContactId)
					if err != nil {
						zlog.Error(err.Error())
						return constants.SYSTEM_ERROR, nil, -1
					}
					userListRsp = append(userListRsp, respond.MyUserListRespond{
						UserId:   user.Uuid,
						UserName: user.Nickname,
						Avatar:   user.Avatar,
					})
				}
			}
			rspString, err := json.Marshal(userListRsp)
			if err != nil {
				zlog.Error(err.Error())
			}
			if err := myredis.SetKeyEx("contact_user_list_"+ownerId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
				zlog.Error(err.Error())
			}
			return "获取用户列表成功", userListRsp, 0
		}
		zlog.Error(err.Error())
	}
	var rsp []respond.MyUserListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取用户列表成功", rsp, 0
}

// LoadMyJoinedGroup 获取我加入的群聊
func (u *userContactService) LoadMyJoinedGroup(ownerId string) (string, []respond.LoadMyJoinedGroupRespond, int) {
	rspString, err := myredis.GetKeyNilIsErr("my_joined_group_list_" + ownerId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			contactList, err := u.userContactDao.GetJoinedGroupContacts(ownerId)
			if err != nil {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			if len(contactList) == 0 {
				message := "目前不存在加入的群聊"
				zlog.Info(message)
				return message, nil, 0
			}
			var groupList []model.GroupInfo
			for _, contact := range contactList {
				if contact.ContactId[0] == 'G' {
					group, err := u.groupDao.GetGroupByUUID(contact.ContactId)
					if err != nil {
						zlog.Error(err.Error())
						return constants.SYSTEM_ERROR, nil, -1
					}
					if group.OwnerId != ownerId {
						groupList = append(groupList, *group)
					}
				}
			}
			var groupListRsp []respond.LoadMyJoinedGroupRespond
			for _, group := range groupList {
				groupListRsp = append(groupListRsp, respond.LoadMyJoinedGroupRespond{
					GroupId:   group.Uuid,
					GroupName: group.Name,
					Avatar:    group.Avatar,
				})
			}
			rspString, err := json.Marshal(groupListRsp)
			if err != nil {
				zlog.Error(err.Error())
			}
			if err := myredis.SetKeyEx("my_joined_group_list_"+ownerId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
				zlog.Error(err.Error())
			}
			return "获取加入群成功", groupListRsp, 0
		}
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}
	var rsp []respond.LoadMyJoinedGroupRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取加入群成功", rsp, 0
}

// GetContactInfo 获取联系人信息
// 调用这个接口的前提是该联系人没有处在删除或被删除，或者该用户还在群聊中
// redis todo
func (u *userContactService) GetContactInfo(contactId string) (string, respond.GetContactInfoRespond, int) {
	if contactId[0] == 'G' {
		group, err := u.groupDao.GetGroupByUUID(contactId)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, respond.GetContactInfoRespond{}, -1
		}
		if group.Status != group_status_enum.DISABLE {
			return "获取联系人信息成功", respond.GetContactInfoRespond{
				ContactId:        group.Uuid,
				ContactName:      group.Name,
				ContactAvatar:    group.Avatar,
				ContactNotice:    group.Notice,
				ContactAddMode:   group.AddMode,
				ContactMembers:   group.Members,
				ContactMemberCnt: group.MemberCnt,
				ContactOwnerId:   group.OwnerId,
			}, 0
		}
		zlog.Error("该群聊处于禁用状态")
		return "该群聊处于禁用状态", respond.GetContactInfoRespond{}, -2
	}

	user, err := u.userDao.GetUserByUUID(contactId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, respond.GetContactInfoRespond{}, -1
	}
	log.Println(user)
	if user.Status != user_status_enum.DISABLE {
		return "获取联系人信息成功", respond.GetContactInfoRespond{
			ContactId:        user.Uuid,
			ContactName:      user.Nickname,
			ContactAvatar:    user.Avatar,
			ContactBirthday:  user.Birthday,
			ContactEmail:     user.Email,
			ContactPhone:     user.Telephone,
			ContactGender:    user.Gender,
			ContactSignature: user.Signature,
		}, 0
	}
	zlog.Info("该用户处于禁用状态")
	return "该用户处于禁用状态", respond.GetContactInfoRespond{}, -2
}

// DeleteContact 删除联系人（只包含用户）
func (u *userContactService) DeleteContact(ownerId, contactId string) (string, int) {
	deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}
	if err := u.userContactDao.DeleteContactCascade(ownerId, contactId, deletedAt); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if err := myredis.DelKeysWithPattern("contact_user_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}
	return "删除联系人成功", 0
}

// ApplyContact 申请添加联系人
func (u *userContactService) ApplyContact(req request.ApplyContactRequest) (string, int) {
	if req.ContactId[0] == 'U' {
		user, err := u.userDao.GetUserByUUID(req.ContactId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				zlog.Error("用户不存在")
				return "用户不存在", -2
			}
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		if user.Status == user_status_enum.DISABLE {
			zlog.Info("用户已被禁用")
			return "用户已被禁用", -2
		}
		contactApply, err := u.userContactDao.GetContactApply(req.OwnerId, req.ContactId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				contactApply = &model.ContactApply{
					Uuid:        fmt.Sprintf("A%s", random.GetNowAndLenRandomString(11)),
					UserId:      req.OwnerId,
					ContactId:   req.ContactId,
					ContactType: contact_type_enum.USER,
					Status:      contact_apply_status_enum.PENDING,
					Message:     req.Message,
					LastApplyAt: time.Now(),
				}
				if err := u.userContactDao.CreateContactApply(contactApply); err != nil {
					zlog.Error(err.Error())
					return constants.SYSTEM_ERROR, -1
				}
			} else {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
		if contactApply.Status == contact_apply_status_enum.BLACK {
			return "对方已将你拉黑", -2
		}
		contactApply.LastApplyAt = time.Now()
		contactApply.Status = contact_apply_status_enum.PENDING
		if err := u.userContactDao.UpdateContactApply(contactApply); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		return "申请成功", 0
	}
	if req.ContactId[0] == 'G' {
		group, err := u.groupDao.GetGroupByUUID(req.ContactId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				zlog.Error("群聊不存在")
				return "群聊不存在", -2
			}
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		if group.Status == group_status_enum.DISABLE {
			zlog.Info("群聊已被禁用")
			return "群聊已被禁用", -2
		}
		contactApply, err := u.userContactDao.GetContactApply(req.OwnerId, req.ContactId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				contactApply = &model.ContactApply{
					Uuid:        fmt.Sprintf("A%s", random.GetNowAndLenRandomString(11)),
					UserId:      req.OwnerId,
					ContactId:   req.ContactId,
					ContactType: contact_type_enum.GROUP,
					Status:      contact_apply_status_enum.PENDING,
					Message:     req.Message,
					LastApplyAt: time.Now(),
				}
				if err := u.userContactDao.CreateContactApply(contactApply); err != nil {
					zlog.Error(err.Error())
					return constants.SYSTEM_ERROR, -1
				}
			} else {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
		contactApply.LastApplyAt = time.Now()
		if err := u.userContactDao.UpdateContactApply(contactApply); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		return "申请成功", 0
	}
	return "用户/群聊不存在", -2
}

// GetNewContactList 获取新的联系人申请列表
func (u *userContactService) GetNewContactList(ownerId string) (string, []respond.NewContactListRespond, int) {
	contactApplyList, err := u.userContactDao.GetContactApplyListByContactAndStatus(ownerId, contact_apply_status_enum.PENDING)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			zlog.Info("没有在申请的联系人")
			return "没有在申请的联系人", nil, 0
		}
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}
	if len(contactApplyList) == 0 {
		zlog.Info("没有在申请的联系人")
		return "没有在申请的联系人", nil, 0
	}
	var rsp []respond.NewContactListRespond
	for _, contactApply := range contactApplyList {
		message := "申请理由：无"
		if contactApply.Message != "" {
			message = "申请理由：" + contactApply.Message
		}
		newContact := respond.NewContactListRespond{
			ContactId: contactApply.Uuid,
			Message:   message,
		}
		user, err := u.userDao.GetUserByUUID(contactApply.UserId)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
		newContact.ContactId = user.Uuid
		newContact.ContactName = user.Nickname
		newContact.ContactAvatar = user.Avatar
		rsp = append(rsp, newContact)
	}
	return "获取成功", rsp, 0
}

// GetAddGroupList 获取新的加群列表
// 前端已经判断调用接口的用户是群主，也只有群主才能调用这个接口
func (u *userContactService) GetAddGroupList(groupId string) (string, []respond.AddGroupListRespond, int) {
	contactApplyList, err := u.userContactDao.GetContactApplyListByContactAndStatus(groupId, contact_apply_status_enum.PENDING)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			zlog.Info("没有在申请的联系人")
			return "没有在申请的联系人", nil, 0
		}
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}
	if len(contactApplyList) == 0 {
		zlog.Info("没有在申请的联系人")
		return "没有在申请的联系人", nil, 0
	}
	var rsp []respond.AddGroupListRespond
	for _, contactApply := range contactApplyList {
		message := "申请理由：无"
		if contactApply.Message != "" {
			message = "申请理由：" + contactApply.Message
		}
		newContact := respond.AddGroupListRespond{
			ContactId: contactApply.Uuid,
			Message:   message,
		}
		user, err := u.userDao.GetUserByUUID(contactApply.UserId)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
		newContact.ContactId = user.Uuid
		newContact.ContactName = user.Nickname
		newContact.ContactAvatar = user.Avatar
		rsp = append(rsp, newContact)
	}
	return "获取成功", rsp, 0
}

// PassContactApply 通过联系人申请
func (u *userContactService) PassContactApply(ownerId string, contactId string) (string, int) {
	contactApply, err := u.userContactDao.GetContactApply(contactId, ownerId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if ownerId[0] == 'U' {
		user, err := u.userDao.GetUserByUUID(contactId)
		if err != nil {
			zlog.Error(err.Error())
		}
		if user.Status == user_status_enum.DISABLE {
			zlog.Error("用户已被禁用")
			return "用户已被禁用", -2
		}
		contactApply.Status = contact_apply_status_enum.AGREE
		if err := u.userContactDao.UpdateContactApply(contactApply); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		newContact := model.UserContact{
			UserId:      ownerId,
			ContactId:   contactId,
			ContactType: contact_type_enum.USER,
			Status:      contact_status_enum.NORMAL,
			CreatedAt:   time.Now(),
			UpdateAt:    time.Now(),
		}
		if err := u.userContactDao.CreateUserContact(&newContact); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		anotherContact := model.UserContact{
			UserId:      contactId,
			ContactId:   ownerId,
			ContactType: contact_type_enum.USER,
			Status:      contact_status_enum.NORMAL,
			CreatedAt:   newContact.CreatedAt,
			UpdateAt:    newContact.UpdateAt,
		}
		if err := u.userContactDao.CreateUserContact(&anotherContact); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}
		if err := myredis.DelKeysWithPattern("contact_user_list_" + ownerId); err != nil {
			zlog.Error(err.Error())
		}
		return "已添加该联系人", 0
	}

	group, err := u.groupDao.GetGroupByUUID(ownerId)
	if err != nil {
		zlog.Error(err.Error())
	}
	if group.Status == group_status_enum.DISABLE {
		zlog.Error("群聊已被禁用")
		return "群聊已被禁用", -2
	}
	contactApply.Status = contact_apply_status_enum.AGREE
	if err := u.userContactDao.UpdateContactApply(contactApply); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	newContact := model.UserContact{
		UserId:      contactId,
		ContactId:   ownerId,
		ContactType: contact_type_enum.GROUP,
		Status:      contact_status_enum.NORMAL,
		CreatedAt:   time.Now(),
		UpdateAt:    time.Now(),
	}
	if err := u.userContactDao.CreateUserContact(&newContact); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	var members []string
	if err := json.Unmarshal(group.Members, &members); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	members = append(members, contactId)
	group.MemberCnt = len(members)
	group.Members, _ = json.Marshal(members)
	if err := u.groupDao.SaveGroup(group); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if err := myredis.DelKeysWithPattern("my_joined_group_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}
	return "已通过加群申请", 0
}

// RefuseContactApply 拒绝联系人申请
func (u *userContactService) RefuseContactApply(ownerId string, contactId string) (string, int) {
	contactApply, err := u.userContactDao.GetContactApply(contactId, ownerId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	contactApply.Status = contact_apply_status_enum.REFUSE
	if err := u.userContactDao.UpdateContactApply(contactApply); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if ownerId[0] == 'U' {
		return "已拒绝该联系人申请", 0
	}
	return "已拒绝该加群申请", 0
}

// BlackContact 拉黑联系人
func (u *userContactService) BlackContact(ownerId string, contactId string) (string, int) {
	deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}
	if err := u.userContactDao.BlackContactCascade(ownerId, contactId, deletedAt, time.Now()); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	return "已拉黑该联系人", 0
}

// CancelBlackContact 取消拉黑联系人
func (u *userContactService) CancelBlackContact(ownerId string, contactId string) (string, int) {
	blackContact, err := u.userContactDao.GetUserContact(ownerId, contactId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if blackContact.Status != contact_status_enum.BLACK {
		return "未拉黑该联系人，无需解除拉黑", -2
	}
	beBlackContact, err := u.userContactDao.GetUserContact(contactId, ownerId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if beBlackContact.Status != contact_status_enum.BE_BLACK {
		return "该联系人未被拉黑，无需解除拉黑", -2
	}
	if err := u.userContactDao.CancelBlackContact(ownerId, contactId, time.Now()); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	return "已解除拉黑该联系人", 0
}

// BlackApply 拉黑申请
func (u *userContactService) BlackApply(ownerId string, contactId string) (string, int) {
	contactApply, err := u.userContactDao.GetContactApply(contactId, ownerId)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	contactApply.Status = contact_apply_status_enum.BLACK
	if err := u.userContactDao.UpdateContactApply(contactApply); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	return "已拉黑该申请", 0
}
