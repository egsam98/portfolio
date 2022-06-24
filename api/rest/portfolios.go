package rest

import (
	"github.com/egsam98/portfolio/api/rest/requests"
	"github.com/egsam98/portfolio/domain/portfolio"
	"github.com/labstack/echo/v4"
)

type portfoliosController struct {
	pm *portfolio.Manager
}

func newPortfoliosController(pm *portfolio.Manager) *portfoliosController {
	return &portfoliosController{pm: pm}
}

// getData godoc
// @Router /portfolios/:name/data [get]
// @Summary Portfolio data
// @Tags Portfolios
// @Param name path string true "Portfolio name"
// @Produce json
// @Success	200 {object} portfolio.Info
// @Failure 400 {object} echo.HTTPError
func (p *portfoliosController) getData(ctx echo.Context) error {
	name := ctx.Param("name")
	portf, err := p.pm.Portfolio(name)
	if err != nil {
		return err
	}

	info, err := portf.Info(ctx.Request().Context())
	if err != nil {
		return err
	}
	return ctx.JSON(200, info)
}

// addTriggers godoc
// @Router /portfolios/:name/triggers [post]
// @Summary Add trigger to portfolio
// @Tags Portfolios
// @Param name path string true "Portfolio name"
// @Param body body requests.AddTriggers true " "
// @Accept json
// @Produce json
// @Success	200 {object} portfolio.TriggerSettings
// @Failure 400 {object} echo.HTTPError
func (p *portfoliosController) addTriggers(ctx echo.Context) error {
	name := ctx.Param("name")

	var req requests.AddTriggers
	if err := ctx.Bind(&req); err != nil {
		return err
	}

	portf, err := p.pm.Portfolio(name)
	if err != nil {
		return err
	}

	triggers := make([]portfolio.Trigger, 0, len(req))
	for _, elem := range req {
		var trigger portfolio.Trigger
		switch elem.Type {
		case portfolio.CCBP:
			trigger = portfolio.NewCostChangedByPercent(portf, elem.Currency, *elem.Percent, elem.TrailingAlert)
		case portfolio.CRL:
			trigger = portfolio.NewCostReachedLimit(portf, elem.Currency, *elem.Limit)
		default:
			continue
		}
		triggers = append(triggers, trigger)
	}

	settings, err := portf.AddTriggers(ctx.Request().Context(), triggers)
	if err != nil {
		return err
	}
	return ctx.JSON(200, settings)
}
