package delete

import (
	"fmt"
	"github.com/devtron-labs/devtron/internal/sql/repository/app"
	"github.com/devtron-labs/devtron/internal/sql/repository/pipelineConfig"
	"github.com/devtron-labs/devtron/pkg/cluster"
	"github.com/devtron-labs/devtron/pkg/cluster/repository"
	"github.com/devtron-labs/devtron/pkg/team"
	"github.com/go-pg/pg"
	"go.uber.org/zap"
)

type DeleteServiceExtendedImpl struct {
	appRepository         app.AppRepository
	environmentRepository repository.EnvironmentRepository
	pipelineRepository    pipelineConfig.PipelineRepository
	*DeleteServiceImpl
}

func NewDeleteServiceExtendedImpl(logger *zap.SugaredLogger,
	teamService team.TeamService,
	clusterService cluster.ClusterService,
	environmentService cluster.EnvironmentService,
	appRepository app.AppRepository,
	environmentRepository repository.EnvironmentRepository,
	pipelineRepository pipelineConfig.PipelineRepository,
) *DeleteServiceExtendedImpl {
	return &DeleteServiceExtendedImpl{
		appRepository:         appRepository,
		environmentRepository: environmentRepository,
		pipelineRepository:    pipelineRepository,
		DeleteServiceImpl: &DeleteServiceImpl{
			logger:             logger,
			teamService:        teamService,
			clusterService:     clusterService,
			environmentService: environmentService,
		},
	}
}

func (impl DeleteServiceExtendedImpl) DeleteCluster(deleteRequest *cluster.ClusterBean, userId int32) error {
	//finding if there are env in this cluster or not, if yes then will not delete
	env, err := impl.environmentRepository.FindByClusterId(deleteRequest.Id)
	if !(env == nil && err == pg.ErrNoRows) {
		impl.logger.Errorw("err in deleting cluster, found env in this cluster", "clusterName", deleteRequest.ClusterName, "err", err)
		return fmt.Errorf(" Please delete all related environments before deleting this cluster : %w", err)
	}
	err = impl.clusterService.DeleteFromDb(deleteRequest, userId)
	if err != nil {
		impl.logger.Errorw("error im deleting cluster", "err", err, "deleteRequest", deleteRequest)
		return err
	}
	return nil
}

func (impl DeleteServiceExtendedImpl) DeleteEnvironment(deleteRequest *cluster.EnvironmentBean, userId int32) error {
	//finding if this env is used in any cd pipelines, if yes then will not delete
	pipelines, err := impl.pipelineRepository.FindActiveByEnvId(deleteRequest.Id)
	if !(pipelines == nil && err == pg.ErrNoRows) {
		impl.logger.Errorw("err in deleting env, found pipelines in this env", "envName", deleteRequest.Environment, "err", err)
		return fmt.Errorf(" Please delete all related pipelines before deleting this environment : %w", err)
	}
	err = impl.environmentService.Delete(deleteRequest, userId)
	if err != nil {
		impl.logger.Errorw("error in deleting environment", "err", err, "deleteRequest", deleteRequest)
	}
	return nil
}
func (impl DeleteServiceExtendedImpl) DeleteTeam(deleteRequest *team.TeamRequest) error {
	//finding if this project is used in some app; if yes, will not perform delete operation
	apps, err := impl.appRepository.FindAppsByTeamName(deleteRequest.Name)
	if !(apps == nil && err == pg.ErrNoRows) {
		impl.logger.Errorw("err in deleting team, found apps in team", "teamName", deleteRequest.Name, "err", err)
		return fmt.Errorf(" Please delete all apps in this project before deleting this project : %w", err)
	}
	err = impl.teamService.Delete(deleteRequest)
	if err != nil {
		impl.logger.Errorw("error in deleting team", "err", err, "deleteRequest", deleteRequest)
		return err
	}
	return nil
}
