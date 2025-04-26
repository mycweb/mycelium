# Mycelium Network Protocol (MNP)
MNP is a message passing protocol for 2 parties to exchange Mycelium Values.

MNP depends on a transport which must provide the following:
  - `Tell(dst, msg)`         Send a msg to the other party without receiving a reply.
  - `Ask(dst, msg) -> msg`   Send a msg to the other party and block until a corresponding response is received.

## Message Format
The transport layer presents sized messages to MNP, so messages do not need to contain length headers.
Messages have a 1 byte for the message type, followed by an arbitrary amount of bytes, which will be validated based on the message type.

## Message Types

### `Blob Pull`
This message contains a single Ref.
It is used to request a Blob from the other party's Store.

### `Blob Push`
Blob Push is sent in response to a a BlobPull message.
It must contain data which hashes to the value requested.

### `TellAnyValue`
Sends a single *AnyValue* with Tell semantics to the remote peer.
The remote peer can use BlobPull messages to resolve references as needed.

### `AskAnyValue`
Sends a single AnyValue with Ask semantics to the remote peer.
The response must have type ReplyAnyValue.

### `ReplyAnyValue`
This is sent in response to an AskAnyValue.  It contains a single Mycelium AnyValue.

## Peers
All communication in MNP is encrypted and authenticated.
The Peer Type is defined as `Distinct[base=AnyValue, mark="mycelium-network.Peer"]`, the AnyValue will contain a PublicKey.

Peers are often identified using their `PeerID`, which is the Mycelium Fingerprint of the Peer.