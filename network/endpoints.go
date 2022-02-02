package network

var (
	PingEndpoint                  = "/ping"
	GetLatestTimestampsEndpoint   = "/timestamps"
	GetPrefixTreeRootHashEndpoint = "/prefix_tree_root"
	GetStorageCostEndpoint        = "/storage_cost"

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

	CreateTableWithoutRandomMissingEndpoint                  = "/create_table_no_missing"
	CreateTableWithRandomMissingEndpoint                     = "/create_table_missing"
	CreateTableWithRandomMissingForExperimentEndpoint        = "/create_table_missing_exp"
	RegenerateTableWithRandomMissingForExperimentEndpoint    = "/regenerate_table"
	RegenerateTableWithoutRandomMissingForExperimentEndpoint = "/regenerate_table_no_miss"
	CreateExample2TableEndpoint                              = "/create_eg_table"

	ResetDurationsEndpoint = "/reset_duration"
)
