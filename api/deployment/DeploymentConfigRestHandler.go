package deployment

import (
	"github.com/devtron-labs/devtron/api/restHandler/common"
	chartRepoRepository "github.com/devtron-labs/devtron/pkg/chartRepo/repository"
	"github.com/devtron-labs/devtron/pkg/pipeline"
	"github.com/devtron-labs/devtron/pkg/sql"
	"github.com/devtron-labs/devtron/pkg/user"
	"github.com/devtron-labs/devtron/pkg/user/casbin"
	"github.com/juju/errors"
	"go.uber.org/zap"
	"gopkg.in/go-playground/validator.v9"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type DeploymentConfigRestHandler interface {
	CreateChartFromFile(w http.ResponseWriter, r *http.Request)
}

type DeploymentConfigRestHandlerImpl struct {
	Logger             *zap.SugaredLogger
	userAuthService    user.UserService
	enforcer           casbin.Enforcer
	validator          *validator.Validate
	refChartDir        pipeline.RefChartDir
	chartService       pipeline.ChartService
	chartRefRepository chartRepoRepository.ChartRefRepository
}

func NewDeploymentConfigRestHandlerImpl(Logger *zap.SugaredLogger, userAuthService user.UserService, enforcer casbin.Enforcer, validator *validator.Validate,
	refChartDir pipeline.RefChartDir, chartService pipeline.ChartService, chartRefRepository chartRepoRepository.ChartRefRepository) *DeploymentConfigRestHandlerImpl {
	return &DeploymentConfigRestHandlerImpl{
		Logger:             Logger,
		userAuthService:    userAuthService,
		enforcer:           enforcer,
		validator:          validator,
		refChartDir:        refChartDir,
		chartService:       chartService,
		chartRefRepository: chartRefRepository,
	}
}

func (handler *DeploymentConfigRestHandlerImpl) CreateChartFromFile(w http.ResponseWriter, r *http.Request) {
	userId, err := handler.userAuthService.GetLoggedInUser(r)
	if userId == 0 || err != nil {
		common.WriteJsonResp(w, err, nil, http.StatusUnauthorized)
		return
	}

	token := r.Header.Get("token")
	if ok := handler.enforcer.Enforce(token, casbin.ResourceGlobal, casbin.ActionUpdate, "*"); !ok {
		common.WriteJsonResp(w, errors.New("unauthorized"), nil, http.StatusForbidden)
		return
	}

	file, fileHeader, err := r.FormFile("BinaryFile")
	if err != nil {
		handler.Logger.Errorw("request err, File parsing error", "err", err, "payload", file)
		common.WriteJsonResp(w, err, nil, http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		handler.Logger.Errorw("request err, Corrupted form data", "err", err, "payload", file)
		common.WriteJsonResp(w, err, nil, http.StatusBadRequest)
		return
	}

	err = handler.chartService.ValidateUploadedFileFormat(fileHeader.Filename)
	if err != nil {
		handler.Logger.Errorw("request err, Unsupported format", "err", err, "payload", file)
		common.WriteJsonResp(w, errors.New("Unsupported format file is uploaded, please upload file with .tar.gz extension"), nil, http.StatusBadRequest)
		return
	}

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		handler.Logger.Errorw("request err, File parsing error", "err", err, "payload", file)
		common.WriteJsonResp(w, err, nil, http.StatusBadRequest)
		return
	}

	chartInfo, err := handler.chartService.ExtractChartIfMissing(fileBytes, string(handler.refChartDir), "")

	if chartInfo.TemporaryFolder != "" {
		err1 := os.RemoveAll(chartInfo.TemporaryFolder)
		if err1 != nil {
			handler.Logger.Errorw("error in deleting temp dir ", "err", err)
		}
	}

	if err != nil {
		common.WriteJsonResp(w, err, nil, http.StatusBadRequest)
		return
	}

	chartRefs := &chartRepoRepository.ChartRef{
		Name:      chartInfo.ChartName,
		Version:   chartInfo.ChartVersion,
		Location:  chartInfo.ChartLocation,
		Active:    true,
		Default:   false,
		ChartData: fileBytes,
		AuditLog: sql.AuditLog{
			CreatedBy: userId,
			CreatedOn: time.Now(),
			UpdatedOn: time.Now(),
			UpdatedBy: userId,
		},
	}

	err = handler.chartRefRepository.Save(chartRefs)
	if err != nil {
		handler.Logger.Errorw("error in saving ConfigMap, CallbackConfigMap", "err", err)
		common.WriteJsonResp(w, err, "Chart couldn't be saved", http.StatusInternalServerError)
		return
	}
	common.WriteJsonResp(w, err, "Chart Saved Successfully", http.StatusOK)
	return
}
