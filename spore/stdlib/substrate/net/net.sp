;; package net deals with communication
(defl localAddr {x: NodeInfo} ((!field 0)))

(defl messageAddr {x: Message} Addr
    (!field x 0)
)

(defl messagePayload {x: Message} Any
    (!field x 1)
)

(defc Node (Ref (Product
    (Lambda () NodeInfo) ;; Info 
    (Lambda Unit Message) ;; Receive
    (Lambda (Product Message) Unit) ;; Tell
)))

(pub NodeInfo Message Addr)
(pub Node)

