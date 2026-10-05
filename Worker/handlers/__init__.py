
import pkgutil
import importlib

package_dir = __path__
for _, module_name, _ in pkgutil.iter_modules(package_dir):
    if module_name != "registry":
        importlib.import_module(f'{__name__}.{module_name}')

from .register import get_handler
