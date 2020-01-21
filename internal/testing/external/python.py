## dependencies:
#
# git clone https://github.com/certifi/python-certifi.git
# git clone https://github.com/getsentry/sentry-python.git

import os

from sentry_sdk import init, capture_message, flush, Hub


init(debug=True)

for i in range(int(os.environ["TEST_N"])):
    capture_message("hello")

flush()
# Hub.current.client.close()
