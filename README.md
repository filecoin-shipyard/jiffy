# :zap: **Jiffy**

> **Just In-Time Filecoin For You**

![Go Reference](https://pkg.go.dev/badge/github.com/filecoin-shipyard/jiffy.svg)
![Go Test](https://github.com/filecoin-shipyard/jiffy/actions/workflows/go-test.yml/badge.svg)

Jiffy provides a seamless interface to store and retrieve arbitrary data blobs on the Filecoin network. It offers an efficient data preparation system, ensuring data is scanned just once before being prepped for a Filecoin deal. Each uploaded blob in Jiffy is identified with a unique piece CID, which can be leveraged for merkle inclusion proofs. Additionally, Jiffy's bin-packing mechanism optimizes storage by fitting blobs into full Filecoin sectors and ensures data integrity by checking both the storage provider and the Filecoin chain.

Its API is designed to be fully compatible with [Motion](https://github.com/filecoin-project/motion) and includes an integrated [Motion blob store](integration/motion).

## **Features**

- **Arbitrary Blob Management**: Easily store `io.Reader` data on Filecoin and retrieve it as `io.ReadSeekCloser`.
- **One-Pass Data Preparation**: Processes input data in a single pass for Filecoin storage.
- **Unique Piece CID for Blobs**: Every stored blob gets its own Piece CID for use in merkle inclusion proofs.
- **Optimized Bin Packing**: Efficiently packs blobs across sectors to ensure cost-effective storage on Filecoin.
- **Customizable Replication**: Decide the storage providers and the replication factor for each piece.
- **Integrated with Motion Blob Store**: Contains a built-in [Motion blob store](integration/motion) implementation.
- **Efficient Byte-Range Retrieval** (*WIP*): Retrieves data with Boost Piece CID range request and performs on-the-fly content verification.
- **HTTP Server Offloading** (*WIP*): Routes data to a local HTTP server, authenticated using per deal proposal JWT.
- **S3-Compatible Offloading** (*WIP*): Allows data routing to any S3-compatible API.

> [//]: # (TODO: Add a comparative table between Jiffy, RIBS, and Singularity)
>
> [//]: # (TODO: Write a section on singularity v3)

## **Development Status**

:construction: **Under Active Development** :construction:

We aim to keep the `main` branch stable. However, as this is an evolving project, breaking changes might occur. For production environments, please rely on released versions and always consult the release notes before upgrading.

## **Documentation**

For an in-depth understanding and integration specifics, refer to the [godoc documentation](https://pkg.go.dev/github.com/filecoin-shipyard/jiffy).

## **License**

This project is dual-licensed under the MIT and Apache 2.0 licenses. For more details, consult the [LICENSE.md](LICENSE.md) file.
