package models

type TaskHost struct {
	Id     int   `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskId int   `json:"task_id" gorm:"not null;index"`
	HostId int16 `json:"host_id" gorm:"type:smallint;not null;index"`
}

type TaskHostDetail struct {
	TaskHost
	Name  string `json:"name"`
	Port  int    `json:"port"`
	Alias string `json:"alias"`
}

func (TaskHostDetail) TableName() string {
	return TablePrefix + "task_host"
}

func (th *TaskHost) Remove(taskId int) error {
	return Db.Where("task_id = ?", taskId).Delete(&TaskHost{}).Error
}

func (th *TaskHost) Add(taskId int, hostIds []int) error {
	err := th.Remove(taskId)
	if err != nil {
		return err
	}

	taskHosts := make([]TaskHost, len(hostIds))
	for i, value := range hostIds {
		taskHosts[i].TaskId = taskId
		taskHosts[i].HostId = int16(value)
	}

	return Db.Create(&taskHosts).Error
}

func (th *TaskHost) GetHostIdsByTaskId(taskId int) ([]TaskHostDetail, error) {
	list := make([]TaskHostDetail, 0)
	err := Db.Table(TablePrefix+"task_host as th").
		Select("th.id", "th.host_id", "h.alias", "h.name", "h.port").
		Joins("LEFT JOIN "+TablePrefix+"host as h ON th.host_id = h.id").
		Where("th.task_id = ?", taskId).
		Find(&list).Error

	return list, err
}

func (th *TaskHost) GetTaskIdsByHostId(hostId int16) ([]interface{}, error) {
	list := make([]TaskHost, 0)
	err := Db.Select("task_id").Where("host_id = ?", hostId).Find(&list).Error
	if err != nil {
		return nil, err
	}

	taskIds := make([]interface{}, len(list))
	for i, value := range list {
		taskIds[i] = value.TaskId
	}

	return taskIds, err
}

// 判断主机id是否有引用
func (th *TaskHost) HostIdExist(hostId int16) (bool, error) {
	var count int64
	err := Db.Model(&TaskHost{}).Where("host_id = ?", hostId).Count(&count).Error
	return count > 0, err
}
