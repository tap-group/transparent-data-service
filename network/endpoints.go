package network

var (
	PingEndpoint                  = "/ping"
	GetLatestTimestampsEndpoint   = "/timestamps"
	GetPrefixTreeRootHashEndpoint = "/prefix_tree_root"
	GetStorageCostEndpoint        = "/storage_cost"
	GetBuildTreeTimeEndpoint      = "/tree_time"
	GetBuildProofsTimeEindpoint   = "/proofs_time"

	AddToSqlTableEndpoint      = "/add_to_sql"
	InitializeSqlTableEndpoint = "/init_sql"
	AddToTreeEndpoint          = "/add_to_tree"
	InitializeTreeEndpoint     = "/init_tree"

	RequestPrefixMembershipProofEndpoint         = "/membership_proof"
	RequestCommitmentTreeMembershipProofEndpoint = "/commitment_membership_proof"
	RequestCommitmentsAndProofsEndpoint          = "/commitment_proofs"
	RequestPrefixesEndpoint                      = "/prefixes"

	RequestSumAndCountsEndpoint      = "/sum_counts"
	RequestMinAndProofsEndpoint      = "/min_proofs"
	RequestMaxAndProofsEndpoint      = "/max_proofs"
	RequestQuantileAndProofsEndpoint = "/quantile_proofs"

	CreateTableWithoutRandomMissingEndpoint = "/create_table_no_missing"
	CreateTableWithRandomMissingEndpoint    = "/create_table_missing"
	CreateTableForExperimentEndpoint        = "/create_experiment_table"
	RegenerateTableForExperimentEndpoint    = "/regenerate_experiment_table"
	CreateExample2TableEndpoint             = "/create_eg_table"

	ResetDurationsEndpoint = "/reset_duration"
)
