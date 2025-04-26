(import "namespaces")
(import "substrate/net")

(defl getNetwork {env: namespaces.Namespace, k: String} net.Node
    (let {
        dev : (!anyValueTo (namespaces.get env k) NetDev)
    }
        (!post {
            (lambda {} net.NodeInfo
                (!input dev)
            )
            (lambda {} net.Message
                (!panic "not implemented")
            )
            (lambda {msg: net.Message} ()
                (!panic "not implemented")
            )
        })
    )
)

(pub getNetwork)
