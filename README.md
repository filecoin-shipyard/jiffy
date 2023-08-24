# :zap: jiffy

[![Go Reference](https://pkg.go.dev/badge/github.com/filecoin-shipyard/jiffy.svg)](https://pkg.go.dev/github.com/filecoin-shipyard/jiffy)
[![Go Test](https://github.com/filecoin-shipyard/jiffy/actions/workflows/go-test.yml/badge.svg)](https://github.com/filecoin-shipyard/jiffy/actions/workflows/go-test.yml)

> Just In-time Filecoin For You

Jiffy is a system that stores and retrieves arbitrary blobs to/from FileCoin network with highly efficient data preparation mechanism that only scans the data once to prepare data for Filecoin deal making. Jiffy generates piece CID for each uploaded blob, which can then be used as by merkel inclusion proof to prove that a deal indeed contains a given blob. Jiffy also includes a bin-packing mechanism to tightly pack blobs of arbitrary size into 32 GiB Filecoin sectors, and verifies the health of replicas by checking the storage provider and the Filecoin chain.

Jiffy API is fully compatible with [Motion](https://github.com/filecoin-project/motion) and comes with [Motion blob store integration](integration/motion).

## Features

Jiffy features include:

- **Arbitrary Blob storage and retrieval** - Store `io.Reader` onto FileCoin and get `io.ReadSeakCloser` back.
- **Read-once data preparation** - Reads the input data once to prepare it for storage on Filecoin.
- **Piece CID per Blob** - Provides a Piece CID per stored blob which can then be used in merkle inclusion proof to prove that a deal contains a blob.
- **Best-fit bin packing** - Packs blobs as tightly as possible across configurable number of sectors and sector sizes for most cost-effective storage on Filecoin.
- **Extensible replication control** - Choose which SPs and how many SPs a given piece gets replicated on.
- **Motion blob store integration** - Includes [Motion blob store](integration/motion) implementation
- **Efficient byte-range retrieval** - :construction: **WIP** Retrieves data using Boost Piece CID range request and verifies content on the fly when not available locally.
- **Embedded HTTP server offloading** - :construction: **WIP** Offload data to a local HTTP server with per deal proposal JWT authentication.  
- **S3-compatible offloading** - :construction: **WIP**  Offload data to an S3-compatible API.

[//]: # (TODO: Add a table comparring Jiffy, RIBS and Singularity)

[//]: # (TODO: write up on singularity v3)


## Status

:construction: **This repository is under active development.** :construction:

Please be aware that while we strive to keep the master branch stable, breaking changes may be introduced as we push
forward. We recommend using released versions for production systems and always checking the release notes before
updating.

## Documentation

For detailed usage and integration guidance, please refer
to [godoc documentation](https://pkg.go.dev/github.com/filecoin-shipyard/jiffy).

## License

This project is licensed under the MIT and Apache 2.0 Licenses - see the [LICENSE.md](LICENSE.md) file for details.