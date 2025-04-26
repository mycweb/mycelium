(import "namespaces")
(import "substrate")

(defl main {env: namespaces.Namespace} ()
    (substrate.consoleWrite
        (substrate.getConsole env "myconsole")
        "hello world\n"
    )
)

(pub main)
