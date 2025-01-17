package common

var (
    Reset = "\033[0m"
    Blue = "\033[94m"
    Green = "\033[92m"
    Fail = "\033[91m"
)

func RemoveColors() {
    Reset = ""
    Blue = ""
    Green = ""
    Fail = ""
}
