/**
 * Generated by: go/Service.ftl
 */
package impl

import (
	"errors"

	. "github.com/quintans/goSQL/db"
	"github.com/quintans/goSQL/dbx"
	T "github.com/quintans/taskboard/biz/tables"
	"github.com/quintans/taskboard/common/dto"
	"github.com/quintans/taskboard/common/entity"
	"github.com/quintans/taskboard/common/lov"
	"github.com/quintans/taskboard/common/service"
	. "github.com/quintans/toolkit/ext"
	"github.com/quintans/toolkit/web"
	"github.com/quintans/toolkit/web/app"

	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

const FAULT_BIZ = "BIZ"

var (
	AbsentBoardFault     = dbx.NewPersistenceFail(FAULT_BIZ, "The Board no longer exists")
	AbsentLaneFault      = dbx.NewPersistenceFail(FAULT_BIZ, "The Lane no longer exists")
	OptimistickLockFault = dbx.NewOptimisticLockFail("Unable to apply changes due to a concurrent access. Try again.")
)

var _ service.ITaskBoardService = &TaskBoardService{}

func NewTaskBoardService(appCtx *AppCtx) service.ITaskBoardService {
	return new(TaskBoardService)
}

type TaskBoardService struct {
}

func (this *TaskBoardService) WhoAmI(ctx web.IContext) (dto.IdentityDTO, error) {
	p := ctx.GetPrincipal().(Principal)
	identity := dto.IdentityDTO{}
	identity.Id = &p.UserId
	app := ctx.(*AppCtx)
	var name string
	app.Store.Query(T.USER).Column(T.USER_C_NAME).
		Where(T.USER_C_ID.Matches(p.UserId)).
		SelectInto(&name)
	identity.Name = &name
	roles := make([]*string, len(p.Roles))
	for k, role := range p.Roles {
		s := string(role)
		roles[k] = &s
	}
	identity.Roles = roles

	return identity, nil
}

func (this *TaskBoardService) broadcastBoardChange(ctx web.IContext, id int64) error {
	board, err := this.FullyLoadBoardById(ctx, id)
	if err == nil {
		go Poll.Broadcast(fmt.Sprintf("board:%v", id), board)
		return nil
	} else {
		return err
	}
}

func boardVersion(store IDb, boardId int64) (int64, error) {
	// acquires the soft lock version
	var version int64
	ok, err := store.Query(T.BOARD).
		Column(T.BOARD_C_VERSION).
		Where(T.BOARD_C_ID.Matches(boardId)).
		SelectInto(&version)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, AbsentBoardFault
	}
	return version, nil
}

func checkBoardVersion(store IDb, boardId int64, version int64) error {
	// apply lock
	var affected int64
	affected, err := store.Update(T.BOARD).
		Set(T.BOARD_C_VERSION, version+1).
		Where(T.BOARD_C_ID.Matches(boardId), T.BOARD_C_VERSION.Matches(version)).
		Execute()
	if err != nil {
		return err
	}
	// somebody altered this
	if affected == 0 {
		return OptimistickLockFault
	}
	return nil
}

func laneVersion(store IDb, laneId int64) (int64, error) {
	// acquires the soft lock version
	var version int64
	ok, err := store.Query(T.LANE).
		Column(T.LANE_C_VERSION).
		Where(T.LANE_C_ID.Matches(laneId)).
		SelectInto(&version)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, AbsentLaneFault
	}
	return version, nil
}

func checkLaneVersion(store IDb, laneId int64, version int64) error {
	// apply lock
	var affected int64
	affected, err := store.Update(T.LANE).
		Set(T.LANE_C_VERSION, version+1).
		Where(
		T.LANE_C_ID.Matches(laneId), T.LANE_C_VERSION.Matches(version),
	).Execute()
	if err != nil {
		return err
	}
	// somebody altered this
	if affected == 0 {
		return OptimistickLockFault
	}
	return nil
}

func (this *TaskBoardService) FetchBoardUsers(ctx web.IContext, id int64) ([]dto.BoardUserDTO, error) {
	app := ctx.(*AppCtx)
	if err := canAccessBoard(app.Store, app.AsPrincipal(), id); err != nil {
		return nil, err
	}
	var u []dto.BoardUserDTO
	err := app.Store.Query(T.USER).
		Column(T.USER_C_ID, T.USER_C_NAME).
		Inner(T.USER_A_BOARDS).On(T.BOARD_C_ID.Matches(id)).
		Join().
		List(&u)
	if err != nil {
		return nil, err
	}

	return u, nil
}

// param criteria
// return
func (this *TaskBoardService) FetchBoardAllUsers(ctx web.IContext, criteria dto.BoardUserSearchDTO) (app.Page, error) {
	store := ctx.(*AppCtx).Store
	belongs := store.Query(T.BOARD_USER).Alias("b").
		CountAll().
		Where(
		T.BOARD_USER_C_BOARDS_ID.Matches(criteria.BoardId),
		T.BOARD_USER_C_USERS_ID.Matches(T.USER_C_ID.For("u")),
	)

	q := store.Query(T.USER).Alias("u").Column(
		T.USER_C_ID,
		T.USER_C_VERSION,
		T.USER_C_NAME,
	).Column(belongs).As("Belongs")

	if !IsEmpty(criteria.Name) {
		// insenstive case like
		q.Where(T.USER_C_NAME.ILike("%" + *criteria.Name + "%"))
	}
	order := criteria.OrderBy
	if !IsEmpty(order) {
		switch *order {
		case "name":
			q.Order(T.USER_C_NAME)
		}
		applyDirection(q, criteria.Criteria)
	}

	return app.QueryForPage(q, criteria.Criteria, (*dto.BoardUserDTO)(nil), nil)
}

// param criteria
// return
func (this *TaskBoardService) FetchBoards(ctx web.IContext, criteria dto.BoardSearchDTO) (app.Page, error) {
	q := ctx.(*AppCtx).Store.Query(T.BOARD).All()
	p := ctx.GetPrincipal().(Principal)
	// if not admin restrict to boards that the user has access
	if !p.HasRole(lov.ERole_ADMIN) {
		q.Inner(T.BOARD_A_USERS).On(T.USER_C_ID.Matches(p.UserId)).Join()
	}
	if !IsEmpty(criteria.Name) {
		// insenstive case like
		q.Where(T.BOARD_C_NAME.ILike("%" + *criteria.Name + "%"))
	}
	order := criteria.OrderBy
	if !IsEmpty(order) {
		switch *order {
		case "name":
			q.Order(T.BOARD_C_NAME)
		}

		applyDirection(q, criteria.Criteria)
	}

	return app.QueryForPage(q, criteria.Criteria, (*entity.Board)(nil), nil)
}

// param id
// return
func (this *TaskBoardService) FetchBoardById(ctx web.IContext, id int64) (*entity.Board, error) {
	app := ctx.(*AppCtx)
	if err := canAccessBoard(app.Store, app.AsPrincipal(), id); err != nil {
		return nil, err
	}

	return app.GetBoardDAO().FindById(id)
}

func (this *TaskBoardService) FullyLoadBoardById(ctx web.IContext, id int64) (*entity.Board, error) {
	app := ctx.(*AppCtx)
	if err := canAccessBoard(app.Store, app.AsPrincipal(), id); err != nil {
		return nil, err
	}

	var board = new(entity.Board)
	if _, err := app.Store.Query(T.BOARD).All().
		Outer(T.BOARD_A_LANES).OrderBy(T.LANE_C_POSITION).
		Outer(T.LANE_A_TASKS).OrderBy(T.TASK_C_POSITION).
		Outer(T.TASK_A_USER).Include(T.USER_C_ID, T.USER_C_NAME).
		Fetch().
		Where(T.BOARD_C_ID.Matches(id)).
		SelectTree(board); err != nil {
		return nil, err
	}
	return board, nil
}

// param board
// return
func (this *TaskBoardService) SaveBoard(ctx web.IContext, board *entity.Board) (*entity.Board, error) {
	if err := ctx.(*AppCtx).GetBoardDAO().Save(board); err != nil {
		return nil, err
	}
	return board, nil
}

// param idVersion
// return
func (this *TaskBoardService) DeleteBoard(ctx web.IContext, id int64) error {
	myCtx := ctx.(*AppCtx)
	store := myCtx.Store
	// delete all notifications
	subquery := store.Query(T.TASK).
		Distinct().
		Column(T.TASK_C_ID).
		Inner(T.TASK_A_LANE, T.LANE_A_BOARD).On(T.BOARD_C_ID.Matches(id)).
		Join()

	if _, err := store.Delete(T.NOTIFICATION).
		Where(T.NOTIFICATION_C_TASK_ID.In(subquery)).
		Execute(); err != nil {
		return err
	}
	// delete all tasks
	subquery = store.Query(T.LANE).
		Distinct().
		Column(T.LANE_C_ID).
		Inner(T.LANE_A_BOARD).On(T.BOARD_C_ID.Matches(id)).
		Join()

	if _, err := store.Delete(T.TASK).
		Where(T.TASK_C_LANE_ID.In(subquery)).
		Execute(); err != nil {
		return err
	}
	// delete all lanes
	if _, err := store.Delete(T.LANE).
		Where(T.LANE_C_BOARD_ID.Matches(id)).
		Execute(); err != nil {
		return err
	}
	// delete board
	if _, err := store.Delete(T.BOARD).
		Where(T.BOARD_C_ID.Matches(id)).
		Execute(); err != nil {
		return err
	}
	// delete Board
	//_, err := myCtx.GetBoardDAO().DeleteById(id)
	return nil
}

func (this *TaskBoardService) AddLane(ctx web.IContext, boardId int64) error {
	myCtx := ctx.(*AppCtx)
	store := myCtx.Store

	// acquires the soft lock version
	version, err := boardVersion(store, boardId)
	if err != nil {
		return err
	}

	subquery := store.Query(T.LANE).Alias("pos").
		Column(Add(Coalesce(Max(T.LANE_C_POSITION), 0), 1)).
		Where(T.LANE_C_BOARD_ID.Matches(boardId))

	_, err = store.Insert(T.LANE).
		Columns(T.LANE_C_ID,
		T.LANE_C_VERSION,
		T.LANE_C_NAME,
		T.LANE_C_POSITION,
		T.LANE_C_BOARD_ID,
	).Values(nil, 1, "ChangeMe", subquery, boardId).
		Execute()
	if err != nil {
		return err
	}

	// check lock validity
	err = checkBoardVersion(store, boardId, version)
	if err != nil {
		return err
	}

	return this.broadcastBoardChange(ctx, boardId)
}

// param lane
// return
func (this *TaskBoardService) SaveLane(ctx web.IContext, lane *entity.Lane) (*entity.Lane, error) {
	myCtx := ctx.(*AppCtx)
	if err := myCtx.GetLaneDAO().Save(lane); err != nil {
		return nil, err
	}
	if lane.BoardId != nil {
		if err := this.broadcastBoardChange(ctx, *lane.BoardId); err != nil {
			return nil, err
		}
	}
	return lane, nil
}

// if lane is the last one remove all tasks, if not, move all tasks to the previous lane
// param idVersion
// return
func (this *TaskBoardService) DeleteLastLane(ctx web.IContext, boardId int64) error {
	myCtx := ctx.(*AppCtx)
	store := myCtx.Store

	// acquires the soft lock version
	version, err := boardVersion(store, boardId)
	if err != nil {
		return err
	}

	// get all lanes for this board with lower position than the supplyied lane
	var lanes []*entity.Lane
	err = store.Query(T.LANE).All().
		Where(T.LANE_C_BOARD_ID.Matches(boardId)).
		Order(T.LANE_C_POSITION).Desc().
		List(&lanes)
	if err != nil {
		return err
	}

	if len(lanes) == 0 {
		return nil
	}

	lastLane := lanes[0]

	if len(lanes) > 1 {
		previousLane := lanes[1]
		// remove all notification for the last lane
		if _, err := store.Delete(T.NOTIFICATION).
			Where(T.NOTIFICATION_C_LANE_ID.Matches(lastLane.Id)).
			Execute(); err != nil {
			return err
		}

		// transfer all tasks to the previous lane
		_, err = store.
			Update(T.TASK).
			Set(T.TASK_C_LANE_ID, previousLane.Id).
			Where(T.TASK_C_LANE_ID.Matches(lastLane.Id)).
			Execute()
		if err != nil {
			return err
		}

	} else {
		// delete all notifications, since there are no more lanes
		if _, err := store.Delete(T.NOTIFICATION).Execute(); err != nil {
			return err
		}
		// delete all tasks
		if _, err := store.Delete(T.TASK).
			Where(T.TASK_C_LANE_ID.Matches(lastLane.Id)).
			Execute(); err != nil {
			return err
		}
	}
	// delete lane
	if _, err := store.Delete(T.LANE).
		Where(T.LANE_C_ID.Matches(lastLane.Id)).
		Execute(); err != nil {
		return err
	}

	// check lock validity
	err = checkBoardVersion(store, boardId, version)
	if err != nil {
		return err
	}

	return this.broadcastBoardChange(ctx, boardId)
}

// param user
// return
func (this *TaskBoardService) SaveUser(ctx web.IContext, user dto.UserDTO) (bool, error) {
	store := ctx.(*AppCtx).Store
	if user.Id == nil {
		id, err := store.Insert(T.USER).
			Set(T.USER_C_VERSION, 1).
			Set(T.USER_C_NAME, user.Name).
			Set(T.USER_C_USERNAME, user.Username).
			Set(T.USER_C_PASSWORD, user.Password).
			Set(T.USER_C_DEAD, app.NOT_DELETED).
			Execute()
		if err != nil {
			return false, err
		}
		// create roles
		role := &entity.Role{
			Kind:   lov.ERole_USER,
			UserId: &id,
		}
		store.Insert(T.ROLE).Submit(role)
		// admin role
		if user.Admin {
			role = &entity.Role{
				Kind:   lov.ERole_ADMIN,
				UserId: &id,
			}
			store.Insert(T.ROLE).Submit(role)
		}
	} else {
		dml := store.Update(T.USER).
			Set(T.USER_C_VERSION, *user.Version+1).
			Set(T.USER_C_NAME, user.Name).
			Set(T.USER_C_USERNAME, user.Username).
			Where(T.USER_C_ID.Matches(*user.Id).
			And(T.USER_C_VERSION.Matches(*user.Version).
			And(T.USER_C_DEAD.Matches(app.NOT_DELETED))))
		if user.Password != nil {
			dml.Set(T.USER_C_PASSWORD, user.Password)
		}
		_, err := dml.Execute()
		if err != nil {
			return false, err
		}
		// get admin role
		var cnt int64
		criteria := T.ROLE_C_USER_ID.Matches(*user.Id).
			And(T.ROLE_C_KIND.Matches(lov.ERole_ADMIN))
		store.Query(T.ROLE).
			CountAll().
			Where(criteria).
			SelectInto(&cnt)

		if user.Admin && cnt == 0 {
			store.Insert(T.ROLE).Submit(&entity.Role{
				Kind:   lov.ERole_ADMIN,
				UserId: user.Id,
			})
		} else if !user.Admin && cnt != 0 {
			// no longer admin
			// if I am the admin, cannot remove myself from admin
			p := ctx.GetPrincipal().(Principal)
			if p.UserId != *user.Id {
				store.Delete(T.ROLE).
					Where(criteria).
					Execute()
			}
		}
	}

	return true, nil
}

// param criteria
// return
func (this *TaskBoardService) FetchUsers(ctx web.IContext, criteria dto.UserSearchDTO) (app.Page, error) {
	store := ctx.(*AppCtx).Store
	admin := store.Query(T.ROLE).Alias("r").
		CountAll().
		Where(
		T.ROLE_C_USER_ID.Matches(T.USER_C_ID.For("u")),
		T.ROLE_C_KIND.Matches(lov.ERole_ADMIN),
	)

	// password field not included
	q := ctx.(*AppCtx).Store.Query(T.USER).Alias("u").Column(
		T.USER_C_ID,
		T.USER_C_VERSION,
		T.USER_C_NAME,
		T.USER_C_USERNAME,
	).Column(admin).As("Admin")
	c := T.USER_C_DEAD.Matches(app.NOT_DELETED)
	if !IsEmpty(criteria.Name) {
		// insenstive case like
		c = c.And(T.USER_C_NAME.ILike("%" + *criteria.Name + "%"))
	}
	q.Where(c)
	order := criteria.OrderBy
	if !IsEmpty(order) {
		switch *order {
		case "name":
			q.Order(T.USER_C_NAME)
		}

		applyDirection(q, criteria.Criteria)
	}

	return app.QueryForPage(q, criteria.Criteria, (*dto.UserDTO)(nil), nil)

}

// param userId
// return
func (this *TaskBoardService) DisableUser(ctx web.IContext, iv dto.IdVersionDTO) error {
	return app.SoftDeleteByIdAndVersion(ctx.(*AppCtx).Store, T.USER, *iv.Id, *iv.Version)
}

// param boardId
// param userId
// return
func (this *TaskBoardService) AddUserToBoard(ctx web.IContext, input service.AddUserToBoardIn) error {
	store := ctx.(*AppCtx).Store
	_, err := store.Insert(T.BOARD_USER).
		Set(T.BOARD_USER_C_BOARDS_ID, input.BoardId).
		Set(T.BOARD_USER_C_USERS_ID, input.UserId).
		Execute()

	return err
}

// param boardId
// param userId
// return
func (this *TaskBoardService) RemoveUserFromBoard(ctx web.IContext, input service.RemoveUserFromBoardIn) error {
	store := ctx.(*AppCtx).Store
	var err error
	_, err = store.Delete(T.BOARD_USER).
		Where(T.BOARD_USER_C_BOARDS_ID.Matches(input.BoardId).
		And(T.BOARD_USER_C_USERS_ID.Matches(input.UserId))).
		Execute()

	return err
}

// param name
// return
func (this *TaskBoardService) SaveUserName(ctx web.IContext, name *string) error {
	myCtx := ctx.(*AppCtx)
	store := myCtx.Store

	p := myCtx.GetPrincipal().(Principal)

	var err error
	_, err = store.Update(T.USER).
		Set(T.USER_C_NAME, name).
		Where(T.USER_C_ID.Matches(p.UserId).And(T.USER_C_VERSION.Matches(p.Version))).
		Execute()

	return err
}

// param oldPwd
// param newPwd
// return
func (this *TaskBoardService) ChangeUserPassword(ctx web.IContext, input service.ChangeUserPasswordIn) (string, error) {
	myCtx := ctx.(*AppCtx)
	store := myCtx.Store

	p := myCtx.GetPrincipal().(Principal)

	r, err := store.Update(T.USER).
		Set(T.USER_C_PASSWORD, input.NewPwd).
		Set(T.USER_C_VERSION, p.Version+1).
		Where(
		And(T.USER_C_ID.Matches(p.UserId),
			T.USER_C_VERSION.Matches(p.Version),
			T.USER_C_PASSWORD.Matches(input.OldPwd)),
	).
		Execute()

	if err == nil && r > 0 {
		p.Version += 1
		return serializePrincipal(p)
	}

	return "", err
}

// param task
// return
func (this *TaskBoardService) SaveTask(ctx web.IContext, task *entity.Task) (*entity.Task, error) {
	app := ctx.(*AppCtx)
	store := app.Store

	var boardId int64
	if task.Id != nil {
		if err := canAccessTask(store, app.AsPrincipal(), *task.Id); err != nil {
			return nil, err
		}

		// save without afecting position and lane
		affected, err := store.Update(T.TASK).Columns(
			T.TASK_C_VERSION,
			T.TASK_C_MODIFICATION,
			T.TASK_C_USER_ID,
			T.TASK_C_TITLE,
			T.TASK_C_DETAIL,
			T.TASK_C_HEAD_COLOR,
			T.TASK_C_BODY_COLOR,
		).Values(
			*task.Version+1,
			time.Now(),
			task.UserId,
			task.Title,
			task.Detail,
			task.HeadColor,
			task.BodyColor,
		).Where(
			T.TASK_C_ID.Matches(task.Id),
			T.TASK_C_VERSION.Matches(task.Version),
		).Execute()
		if err != nil {
			return nil, err
		}
		// somebody altered this
		if affected == 0 {
			return nil, OptimistickLockFault
		}
		*task.Version = *task.Version + 1

		ok, err := store.Query(T.LANE).
			Column(T.LANE_C_BOARD_ID).
			Where(T.LANE_C_ID.Matches(task.LaneId)).
			SelectInto(&boardId)
		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, AbsentBoardFault
		}
	} else {
		// acquires the soft lock version
		version, err := laneVersion(store, *task.LaneId)
		if err != nil {
			return nil, err
		}

		var position int64
		_, err = store.Query(T.TASK).
			Column(Add(Coalesce(Max(T.TASK_C_POSITION), 0), 1)).
			Where(T.TASK_C_LANE_ID.Matches(task.LaneId)).
			SelectInto(&position)
		if err != nil {
			return nil, err
		}
		task.Position = Int64Ptr(position)
		if err := ctx.(*AppCtx).GetTaskDAO().Save(task); err != nil {
			return nil, err
		}

		// check lock validity
		err = checkLaneVersion(store, *task.LaneId, version)
		if err != nil {
			return nil, err
		}

		var ok bool
		ok, err = ctx.(*AppCtx).Store.Query(T.LANE).
			Column(T.LANE_C_BOARD_ID).
			Where(T.LANE_C_ID.Matches(task.LaneId)).
			SelectInto(&boardId)
		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, AbsentBoardFault
		}
	}

	if err := this.broadcastBoardChange(ctx, boardId); err != nil {
		return nil, err
	}

	return task, nil
}

// param idVersion
// return
func (this *TaskBoardService) MoveTask(ctx web.IContext, in service.MoveTaskIn) error {
	app := ctx.(*AppCtx)
	store := app.Store
	if err := canAccessTask(store, app.AsPrincipal(), in.TaskId); err != nil {
		return err
	}

	/*
	 * ORIGIN LANE
	 */
	var oldLaneId, oldPosition int64
	store.Query(T.TASK).Column(
		T.TASK_C_LANE_ID,
		T.TASK_C_POSITION,
	).Where(
		T.TASK_C_ID.Matches(in.TaskId),
	).SelectInto(&oldLaneId, &oldPosition)

	// acquires the soft lock version
	version, err := laneVersion(store, oldLaneId)
	if err != nil {
		return err
	}

	// to delete
	if in.Position == -1 {
		// delete all notifications
		if _, err = store.Delete(T.NOTIFICATION).
			Where(T.NOTIFICATION_C_TASK_ID.Matches(in.TaskId)).
			Execute(); err != nil {
			return err
		}
		// delete task
		if _, err = store.Delete(T.TASK).
			Where(T.TASK_C_ID.Matches(in.TaskId)).
			Execute(); err != nil {
			return err
		}
	} else {
		// puts the task in limbo
		_, err = store.Update(T.TASK).
			Set(T.TASK_C_LANE_ID, nil).
			Where(T.TASK_C_ID.Matches(in.TaskId)).
			Execute()
		if err != nil {
			return err
		}
	}
	// recalculate positions from current position
	// get all task ids by ascending order
	taskIds := make([]int64, 0)
	var taskId int64
	err = store.Query(T.TASK).
		Column(T.TASK_C_ID).
		Where(
		T.TASK_C_LANE_ID.Matches(oldLaneId),
		T.TASK_C_POSITION.Greater(oldPosition),
	).Order(T.TASK_C_POSITION).
		ListSimple(func() {
		taskIds = append(taskIds, taskId)
	}, &taskId)
	if err != nil {
		return err
	}
	// increment position
	for _, id := range taskIds {
		_, err = store.Update(T.TASK).
			Set(T.TASK_C_POSITION, Minus(T.TASK_C_POSITION, AsIs(1))).
			Where(T.TASK_C_ID.Matches(id)).
			Execute()
		if err != nil {
			return err
		}
	}

	// check lock validity
	err = checkLaneVersion(store, oldLaneId, version)
	if err != nil {
		return err
	}

	/*
	 * DESTINATION LANE
	 */
	if in.Position != -1 {
		// acquires the soft lock version
		version, err = laneVersion(store, in.LaneId)
		if err != nil {
			return err
		}

		// recalculate positions from current position
		// get all task ids by descending order
		taskIds := make([]int64, 0)
		var taskId int64
		err = store.Query(T.TASK).
			Column(T.TASK_C_ID).
			Where(
			T.TASK_C_LANE_ID.Matches(in.LaneId),
			T.TASK_C_POSITION.GreaterOrMatch(in.Position),
		).Order(T.TASK_C_POSITION).Desc().
			ListSimple(func() {
			taskIds = append(taskIds, taskId)
		}, &taskId)
		if err != nil {
			return err
		}
		// increment position
		for _, id := range taskIds {
			_, err = store.Update(T.TASK).
				Set(T.TASK_C_POSITION, Add(T.TASK_C_POSITION, AsIs(1))).
				Where(T.TASK_C_ID.Matches(id)).
				Execute()
			if err != nil {
				return err
			}
		}
		// sets the position and the lane of the moving task
		_, err = store.Update(T.TASK).
			Set(T.TASK_C_LANE_ID, in.LaneId).
			Set(T.TASK_C_POSITION, in.Position).
			Where(T.TASK_C_ID.Matches(in.TaskId)).
			Execute()
		if err != nil {
			return err
		}

		// check lock validity
		err = checkLaneVersion(store, in.LaneId, version)
		if err != nil {
			return err
		}
	}

	var boardId int64
	var ok bool
	ok, err = ctx.(*AppCtx).Store.Query(T.LANE).
		Column(T.LANE_C_BOARD_ID).
		Where(T.LANE_C_ID.Matches(in.LaneId)).
		SelectInto(&boardId)
	if err != nil {
		return err
	}

	if !ok {
		return AbsentBoardFault
	}

	// send notifications
	if in.Position != -1 && in.LaneId != oldLaneId && SmtpHost != "" {
		var task entity.Task
		var ok bool
		// fetch only the fields I need
		ok, err = store.Query(T.TASK).All().
			Where(T.TASK_C_ID.Matches(in.TaskId)).
			Inner(T.TASK_A_LANE).Fetch().
			Outer(T.TASK_A_NOTIFICATIONS).
			On(T.NOTIFICATION_C_LANE_ID.Matches(in.LaneId)).Fetch().
			SelectTree(&task)
		// this error will not cause a rollback
		if err == nil {
			var to = make([]string, 0)
			if ok && len(task.Notifications) > 0 {
				for _, v := range task.Notifications {
					to = append(to, v.Email)
					//logger.Infof("===> Notification to %s: The task '%s' is at '%s'.", task.Title, task.Lane.Name)
				}
				go SendMail(to, "TaskBoard", fmt.Sprintf("The task '%s' is at '%s'.", *task.Title, *task.Lane.Name))
			}
		} else {
			logger.Errorf("Unable to send Notifications for task %v.\nError: %s", in.TaskId, err)
		}
	}

	return this.broadcastBoardChange(ctx, boardId)
}

func SendMail(to []string, subject string, msg string) {
	var auth smtp.Auth
	if SmtpPass != "" {
		auth = smtp.PlainAuth(
			"",
			SmtpUser,
			SmtpPass,
			SmtpHost,
		)
	}

	address := fmt.Sprintf("%v:%v", SmtpHost, SmtpPort)

	//	build our message
	body := []byte("Subject: " + subject + "\r\n\r\n" + msg)

	if err := SendMailWithInsecureSkip(
		address,
		auth,
		SmtpFrom,
		to,
		body,
	); err != nil {
		logger.Errorf("Unable to send email to %v.\nError: %s", to, err)
	}
	logger.Debugf("===> Sent email to %s:\n%s", to, string(body))
}

// retrived from: https://groups.google.com/forum/#!topic/golang-nuts/W95PXq99uns
func SendMailWithInsecureSkip(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	/*
		if err = c.Hello("localhost"); err != nil {
			return err
		}
	*/
	if ok, _ := c.Extension("STARTTLS"); ok {
		host, _, _ := net.SplitHostPort(addr)
		var config = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}
		if err = c.StartTLS(config); err != nil {
			return err
		}
	}
	if a != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(a); err != nil {
				return err
			}
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

// param taskId
// return
func (this *TaskBoardService) FetchNotifications(ctx web.IContext, criteria dto.NotificationSearchDTO) (app.Page, error) {
	q := ctx.(*AppCtx).Store.Query(T.NOTIFICATION).
		All().
		Where(T.NOTIFICATION_C_TASK_ID.Matches(criteria.TaskId)).
		Outer(T.NOTIFICATION_A_LANE).
		Fetch()

	order := criteria.OrderBy
	if !IsEmpty(order) {
		switch *order {
		case "email":
			q.Order(T.NOTIFICATION_C_EMAIL)

		case "column":
			q.OrderBy(T.LANE_C_NAME)
		}

		applyDirection(q, criteria.Criteria)
	}

	return app.QueryForPage(q, criteria.Criteria, (*entity.Notification)(nil), nil)
}

// param task
// return
func (this *TaskBoardService) SaveNotification(ctx web.IContext, notification *entity.Notification) (*entity.Notification, error) {
	if err := ctx.(*AppCtx).GetNotificationDAO().Save(notification); err != nil {
		return nil, err
	}
	return notification, nil
}

// param id
// return
func (this *TaskBoardService) DeleteNotification(ctx web.IContext, id int64) error {
	_, err := ctx.(*AppCtx).GetNotificationDAO().DeleteById(id)
	return err
}

func applyDirection(q *Query, criteria app.Criteria) {
	asc := DefBool(criteria.Ascending, true)
	if asc {
		q.Asc()
	} else {
		q.Desc()
	}
}

func canAccessBoard(store IDb, p Principal, boardId int64) error {
	if !p.HasRole(lov.ERole_ADMIN) {
		var id int64
		store.Query(T.BOARD_USER).
			Column(T.BOARD_USER_C_BOARDS_ID).
			Where(
			T.BOARD_USER_C_BOARDS_ID.Matches(boardId),
			T.BOARD_USER_C_USERS_ID.Matches(p.UserId),
		).
			SelectInto(&id)
		if id == 0 {
			return errors.New("You do not have access to this board!")
		}
	}
	return nil
}

func canAccessTask(store IDb, p Principal, boardId int64) error {
	if !p.HasRole(lov.ERole_ADMIN) {
		var id int64
		store.Query(T.TASK).
			Column(T.TASK_C_ID).
			Inner(T.TASK_A_LANE, T.LANE_A_BOARD, T.BOARD_A_USERS).
			On(T.USER_C_ID.Matches(p.UserId)).
			Join().
			Where(T.TASK_C_ID.Matches(boardId)).
			SelectInto(&id)
		if id == 0 {
			return errors.New("You do not have access to this task!")
		}
	}
	return nil
}
