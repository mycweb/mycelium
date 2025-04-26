(import "substrate")

(defl main {env: substrate.Env} ()
    (substrate.consoleWrite
        (substrate.getConsole env "console") 
        "hello world\n"
    )
    {}
)

(pub main)
