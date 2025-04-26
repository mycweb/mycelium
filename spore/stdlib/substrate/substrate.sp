(import "namespaces")
(import "substrate/cells")
(import "substrate/net")

(defc Env namespaces.Namespace)
(pub Env)

(defc Console (Port String Bottom Bottom Bottom))

(defl getConsole {env: namespaces.Namespace, k: String} Console
    (!anyValueTo (namespaces.get env k) Console)
)

(defl consoleWrite {port: Console, x: String} Unit
    (!output port x)
)

(pub Console getConsole consoleWrite)
