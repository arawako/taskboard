/**
 * WARNING: Generated code! do not change!
 * Generated by: go/IService.ftl
 */
package service;

import (
	"github.com/quintans/maze"
	"github.com/quintans/taskboard/go/dto"
	"github.com/quintans/taskboard/go/entity"
	"github.com/quintans/toolkit/web/app"
)

type ITaskBoardService interface {
	
	// return
	WhoAmI(c maze.IContext) (dto.IdentityDTO, error)
	
	// return only the users that belong to board
	//
	// param id	
	// return
	FetchBoardUsers(c maze.IContext, id int64) ([]dto.BoardUserDTO, error)
	
	// return all users and flag them if they belong to board
	//
	// param criteria	
	// return
	FetchBoardAllUsers(c maze.IContext, criteria dto.BoardUserSearchDTO) (app.Page, error)
	
	// param criteria	
	// return
	FetchBoards(c maze.IContext, criteria dto.BoardSearchDTO) (app.Page, error)
	
	// param id	
	// return
	FetchBoardById(c maze.IContext, id int64) (*entity.Board, error)
	
	// param id	
	// return
	FullyLoadBoardById(c maze.IContext, id int64) (*entity.Board, error)
	
	// param board	
	// return
	SaveBoard(c maze.IContext, board *entity.Board) (*entity.Board, error)
	
	// param id	
	// return
	DeleteBoard(c maze.IContext, id int64) error
	
	// param boardId	
	// return
	AddLane(c maze.IContext, boardId int64) error
	
	// param lane	
	// return
	SaveLane(c maze.IContext, lane *entity.Lane) (*entity.Lane, error)
	
	// param boardId	
	// return
	DeleteLastLane(c maze.IContext, boardId int64) error
	
	// param user	
	// return
	SaveUser(c maze.IContext, user dto.UserDTO) (bool, error)
	
	// param criteria	
	// return
	FetchUsers(c maze.IContext, criteria dto.UserSearchDTO) (app.Page, error)
	
	// param user	
	// return
	DisableUser(c maze.IContext, user dto.IdVersionDTO) error
	
	// param boardId	
	// param userId	
	// return
	AddUserToBoard(c maze.IContext, input AddUserToBoardIn) error
	
	// param boardId	
	// param userId	
	// return
	RemoveUserFromBoard(c maze.IContext, input RemoveUserFromBoardIn) error
	
	// param name	
	// return
	SaveUserName(c maze.IContext, name *string) error
	
	// param oldPwd	
	// param newPwd	
	// return
	ChangeUserPassword(c maze.IContext, input ChangeUserPasswordIn) (string, error)
	
	// param task	
	// return
	SaveTask(c maze.IContext, task *entity.Task) (*entity.Task, error)
	
	// param taskId	
	// param laneId	
	// param position	position = -1 is to eliminate
	// return
	MoveTask(c maze.IContext, input MoveTaskIn) error
	
	// param criteria	
	// return
	FetchNotifications(c maze.IContext, criteria dto.NotificationSearchDTO) (app.Page, error)
	
	// if it returns null, it means the notification was not saved due a matching one
	//
	// param notification	
	// return
	SaveNotification(c maze.IContext, notification *entity.Notification) (*entity.Notification, error)
	
	// param id	
	// return
	DeleteNotification(c maze.IContext, id int64) error
}
