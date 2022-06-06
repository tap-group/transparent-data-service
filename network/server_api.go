package network

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/tap-group/tdsvc/server"
	"github.com/tap-group/tdsvc/tables"
)

type ServerAPI struct {
	server  server.IServer
	factory tables.ITableFactory
}

func NewServerAPI(server server.IServer, factory tables.ITableFactory) *ServerAPI {
	return &ServerAPI{
		server:  server,
		factory: factory,
	}
}

func (s *ServerAPI) Serve(addr string) {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	e.POST(PingEndpoint, s.Ping)

	e.POST(GetLatestTimestampsEndpoint, s.GetLatestTimestamps)
	e.POST(GetPrefixTreeRootHashEndpoint, s.GetPrefixTreeRootHash)
	e.POST(GetStorageCostEndpoint, s.GetStorageCost)

	e.POST(AddToSqlTableEndpoint, s.AddToSqlTable)
	e.POST(InitializeSqlTableEndpoint, s.InitializeSqlTable)
	e.POST(AddToTreeEndpoint, s.AddToTree)
	e.POST(InitializeTreeEndpoint, s.InitializeTree)

	e.POST(RequestPrefixMembershipProofEndpoint, s.RequestPrefixMembershipProof)
	e.POST(RequestCommitmentTreeMembershipProofEndpoint, s.RequestCommitmentTreeMembershipProof)
	e.POST(RequestCommitmentsAndProofsEndpoint, s.RequestCommitmentsAndProofs)
	e.POST(RequestPrefixesEndpoint, s.RequestPrefixes)

	e.POST(RequestSumAndCountsEndpoint, s.RequestSumAndCounts)
	e.POST(RequestMinAndProofsEndpoint, s.RequestMinAndProofs)
	e.POST(RequestMaxAndProofsEndpoint, s.RequestMaxAndProofs)
	e.POST(RequestQuantileAndProofsEndpoint, s.RequestQuantileAndProofs)

	e.POST(CreateTableWithoutRandomMissingEndpoint, s.CreateTableWithoutRandomMissing)
	e.POST(CreateTableWithRandomMissingEndpoint, s.CreateTableWithRandomMissing)
	e.POST(CreateTableForExperimentEndpoint,
		s.CreateTableForExperiment)
	e.POST(RegenerateTableForExperimentEndpoint,
		s.RegenerateTableForExperiment)

	e.POST(CreateExample2TableEndpoint, s.CreateExample2Table)

	e.POST(ResetDurationsEndpoint, s.ResetDurations)

	log.Fatal(e.Start(addr))
}

func (s *ServerAPI) Ping(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func (s *ServerAPI) GetLatestTimestamps(c echo.Context) error {
	r0, r1, r2, r3 := s.server.GetLatestTimestamps()
	return c.JSON(http.StatusOK, TimestampsResponse{r0, r1, r2, r3})
}

func (s *ServerAPI) GetPrefixTreeRootHash(c echo.Context) error {
	return c.JSON(http.StatusOK, s.server.GetPrefixTreeRootHash())
}

func (s *ServerAPI) GetStorageCost(c echo.Context) error {
	return c.JSON(http.StatusOK, s.server.GetStorageCost())
}

func (s *ServerAPI) AddToSqlTable(c echo.Context) error {
	var request SqlTableRequest
	if err := c.Bind(&request); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s.server.AddToSqlTable(request.Filename, request.Table))
}

func (s *ServerAPI) InitializeSqlTable(c echo.Context) error {
	var request SqlTableRequest
	if err := c.Bind(&request); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s.server.InitializeSqlTable(request.Filename, request.Table))
}

func (s *ServerAPI) AddToTree(c echo.Context) error {
	var request AddToTreeRequest
	if err := c.Bind(&request); err != nil {
		return err
	}
	s.server.AddToTree(request.Columns, request.Table, request.ValueRange)
	return c.NoContent(http.StatusOK)
}

func (s *ServerAPI) InitializeTree(c echo.Context) error {
	var request InitializeTreeRequest
	if err := c.Bind(&request); err != nil {
		return err
	}
	s.server.InitializeTree(
		request.Columns, request.Table,
		request.UserRowIdx, request.TimeRowIdx, request.DataRowIdx, request.SeedRowIdx,
		request.MiscFieldsIdxs,
	)
	return c.NoContent(http.StatusOK)
}

func (s *ServerAPI) RequestPrefixMembershipProof(c echo.Context) error {
	var request []byte
	if err := c.Bind(&request); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s.server.RequestPrefixMembershipProof(request))
}

func (s *ServerAPI) RequestCommitmentTreeMembershipProof(c echo.Context) error {
	var request []byte
	if err := c.Bind(&request); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s.server.RequestCommitmentTreeMembershipProof(request))
}

func (s *ServerAPI) RequestCommitmentsAndProofs(c echo.Context) error {
	var request []byte
	if err := c.Bind(&request); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s.server.RequestCommitmentsAndProofs(request))
}

func (s *ServerAPI) RequestPrefixes(c echo.Context) error {
	var request []byte
	if err := c.Bind(&request); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s.server.RequestPrefixes(request))
}

func (s *ServerAPI) RequestSumAndCounts(c echo.Context) error {
	var request [][]byte
	if err := c.Bind(&request); err != nil {
		return err
	}
	r0, r1, r2, r3, r4 := s.server.RequestSumAndCounts(request)
	return c.JSON(http.StatusOK, SumAndCountsResponse{r0, r1, r2, r3, r4})
}

func (s *ServerAPI) RequestMinAndProofs(c echo.Context) error {
	var request []byte
	if err := c.Bind(&request); err != nil {
		return err
	}
	r0, r1, r2, r3, r4 := s.server.RequestMinAndProofs(request)
	return c.JSON(http.StatusOK, MinMaxProofResponse{r0, r1, r2, r3, r4})
}

func (s *ServerAPI) RequestMaxAndProofs(c echo.Context) error {
	var request [][]byte
	if err := c.Bind(&request); err != nil {
		return err
	}
	r0, r1, r2, r3, r4 := s.server.RequestMaxAndProofs(request)
	return c.JSON(http.StatusOK, MinMaxProofResponse{r0, r1, r2, r3, r4})
}

func (s *ServerAPI) RequestQuantileAndProofs(c echo.Context) error {
	var request QuantileProofRequest
	if err := c.Bind(&request); err != nil {
		return err
	}
	r0, r1, r2, r3, r4, r5 := s.server.RequestQuantileAndProofs(
		request.Prefixes, request.K, request.Q)
	return c.JSON(http.StatusOK, QuantileProofResponse{r0, r1, r2, r3, r4, r5})
}

func (s *ServerAPI) CreateTableWithoutRandomMissing(c echo.Context) error {
	var request string
	if err := c.Bind(&request); err != nil {
		return err
	}
	s.factory.CreateTableWithoutRandomMissing(request)
	return c.NoContent(http.StatusOK)
}

func (s *ServerAPI) CreateTableWithRandomMissing(c echo.Context) error {
	var request string
	if err := c.Bind(&request); err != nil {
		return err
	}
	s.factory.CreateTableWithRandomMissing(request)
	return c.NoContent(http.StatusOK)
}

func (s *ServerAPI) CreateTableForExperiment(c echo.Context) error {
	var request CreateTableRequest
	if err := c.Bind(&request); err != nil {
		return err
	}
	s.factory.CreateTableForExperiment(
		request.Filename, request.NUsers, request.NDistricts, request.NTimeslots,
		request.StartTime, request.MissFreq, request.IndustryFreq, request.MaxPower, request.Mode, request.Nonce,
	)
	return c.NoContent(http.StatusOK)
}

func (s *ServerAPI) RegenerateTableForExperiment(c echo.Context) error {
	var request RegenerateTableRequest
	if err := c.Bind(&request); err != nil {
		return err
	}
	s.factory.RegenerateTableForExperiment(
		request.Filename, request.NTimeslots,
		request.StartTime, request.MissFreq, request.MaxPower, request.Mode, request.Nonce,
	)
	return c.NoContent(http.StatusOK)
}

func (s *ServerAPI) CreateExample2Table(c echo.Context) error {
	var request string
	if err := c.Bind(&request); err != nil {
		return err
	}
	s.factory.CreateExample2Table(request)
	return c.NoContent(http.StatusOK)
}

func (s *ServerAPI) ResetDurations(c echo.Context) error {
	s.server.ResetDurations()
	return c.NoContent(http.StatusOK)
}
