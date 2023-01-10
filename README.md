# Go-BPU
Transforms raw transactions to BOB format. Port from the original [bpu](https://github.com/interplanaria/bpu)

Since this is intended to be used by low level transactoin parsers dependencies are kept to a bare minimum. It does not include the RPC client functionality that connects to a node to get a raw tx. Its designed to be a fast raw tx to BOB  processor.

There is also a [Typescript version](https://github.con/rohenaz/bpu-ts) which does include the originally RPC functionality.