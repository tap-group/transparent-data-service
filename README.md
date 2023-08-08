# TAP: Transparent Data Services

This repository contains a reference implementation of TAP: Transparent and Privacy-Preserving Data Services [4]. The code in this repository can be used to reproduce the experimental results from Section 6 of [the paper](https://www.usenix.org/system/files/sec23summer_125-reijsbergen-prepub.pdf).

TAP is an _authenticated data structure_ that is built on top of a back-end database, and which provides users with the opportunity to verify the correctness of a wide range of query results -- look-ups, sums, counts, averages, higher statistical (sample) moments, minima, maxima, and quantiles. For efficiency, TAP uses a two-layer architecture with a _prefix tree_ as the top layer and a multitude of _sum trees_ (also called "commit trees" in the code) as the bottom layer. Users _monitor_ their data entries through their clients, and the tree structure is validated by _auditors_. TAP provides a level of security and privacy that exceeds that of related multi-user approaches (e.g., a trusted server), yet it has practical performance at scale despite the additional security guarantees. For illustrative purposes, the repository also contains code for experiments with a related baseline, [Merkle²](https://eprint.iacr.org/2021/453), which is more efficient at look-ups but which does not support more general queries sums as sums and quantiles.

WARNING: This is an academic prototype, and should not be used in applications without code review.

## Description

### Folders

The repository's main folders are as follows:

* _auditor_: this contains the code for the auditor's client in the _auditor.go_ file. The auditor's client checks that the prefix tree root claimed by the server can be rebuilt from the sum tree roots, and that the sum trees are well-formed. The (typically) most expensive function in the auditor's client is CheckRangeProofs, which verifies that a sum tree's leaf nodes are ordered using range proofs.
* _client_: this contains the code for the user's client in the _client.go_ file. The functions for the main query types are LookupQuery, requestAndVerifySumAndCount (for sum, count, and average queries), requestAndVerifyMin, requestAndVerifyMax, and requestAndVerifyQuantile.
* _crypto_: contains the code for cryptographic operations, largely based on the (now-deleted) zkrp library by Morais et al. [3]. In particular, p256 implements operations on the secP256k1 elliptic curve and bulletproofs contains an implementation of Bulletproofs by Bünz et al. [1].
* _msquare_: contains the code for the Merkle² results.
* _network_: contains the specification of remote servers and table creators (which may run on another device).
* _output_: folder for the output tables.
* _server_: this contains the code for the server in the _server.go_ file, which interacts with the MySQL backend and which receives and processes queries from clients and auditors.
* _tables_: this contains the _table_factory.go_ file, which is executed during experiments to create tables with randomly-generated entries.
* _trees_: this is where the code for operations on prefix trees (which is largely based on the reference implementation of Merkle² [2]) and on sum trees are stored.

### Flags 
To start an experiment, use go run . in conjunction with the flag -experimentX, where X=1,...,7. In addition, the -remote flag instructs the code to use a remote server, and the -serverURL flag can be used to override the default URL (i.e., http://localhost:9045).

## How to run

### Dependencies

#### MySQL
Install MySQL Server and create a database with name `tap`.
Make sure the database is accessable with user `root` without a password.

#### Golang
Install golang 1.16 or later.

### Running the repo
Clone this repo.
Make sure gcc has been installed; if not, run the following (Linux)
```bash
sudo apt install build-essential
```
Download dependencies inside the repo directory.
```bash
go mod tidy
```

Run the experiments locally with a specific experiment flag.
```bash
go run . -experiment4
```

To run the experiments with a server-client configuration over a network, MySQL must run on the server side and this repo must be cloned on both server and client.

On the server machine, build and run the service.
```bash
go build ./cmd/dbsvc
./dbsvc
```

On client side, run the experiments with `remote` flag.
```bash
go run . -remote
```
You can specify server url with `serverURL` flag. The default is `http://localhost:9045`.

For more detail on reproducing the experiments, see the paper's [Artifact Appendix](https://secartifacts.github.io/usenixsec2023/results).

## License

This repository uses Apache License 2.0, as found in the [transparent-data-service/LICENSE](https://github.com/tap-group/transparent-data-service/blob/main/LICENSE) file.

This repository contains code from the (now-deleted) [zkrp repository](http://web.archive.org/web/20201111215751/https://github.com/ing-bank/zkrp) by ING Bank for bulletproofs and elliptic curves, and the [MerkleSquare repository](https://github.com/ucbrise/MerkleSquare) by UC Berkeley RISE for prefix trees. The former uses GNU Lesser General Public License v3.0, and the latter uses Apache License 2.0.

## References

[1] B. Bünz, J. Bootle, D. Boneh, A. Poelstra, P. Wuille, and G. Maxwell. Bulletproofs: Short proofs for confidential transactions and more. In IEEE S&P, 2018.

[2] Y. Hu, K. Hooshmand, H. Kalidhindi, S. J. Yang, and R. A. Popa. Merkle²: A low-latency transparency log system. In IEEE S&P, 2021.

[3] E. Morais, T. Koens, C. van Wijk, and A. Koren. A survey on zero knowledge range proofs and applications. SN Applied Sciences, 1(8):946, 2019.

[4] D. Reijsbergen, A. Maw, Z. Yang, T. T. A. Dinh, and J. Zhou. TAP: Transparent and privacy-preserving data services. USENIX Security, 2023.
