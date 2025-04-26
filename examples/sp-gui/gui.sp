(import "substrate")
(import "gui")

(defc GUI (gui.mkGUI
    (lambda {env: substrate.Env ops: gui.OpList} ()
        ;; try adjusting the colors
        (let {
            red:   (b8 0)
            green: (b8 128)
            blue:  (b8 0)
            alpha: (b8 255)
        }
            (gui.append ops (gui.fill (gui.color red green blue alpha)))
        )
    )
))

(pub GUI)
