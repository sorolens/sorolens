import os
import glob

files = glob.glob("apps/indexer/**/main.go", recursive=True)
for file in files:
    with open(file, "r") as f:
        c = f.read()
    if "func main() {" in c and "InitTracer" not in c:
        c = c.replace("func main() {", "func main() {\n\ttp, _ := poller.InitTracer()\n\tif tp != nil { defer tp.Shutdown(context.Background()) }\n")
        if '"github.com/sorolens/sorolens/apps/indexer/poller"' not in c:
            c = c.replace('import (', 'import (\n\t"github.com/sorolens/sorolens/apps/indexer/poller"\n')
        with open(file, "w") as f:
            f.write(c)
