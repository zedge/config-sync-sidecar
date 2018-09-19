import logging
import glob
import os
import sys
import tempfile
import kubernetes
from kubernetes.client.models.v1_config_map import V1ConfigMap

from stackdriver import StackdriverJsonFormatter


class SidecarConfig:
    def __init__(self, env):
        """
        :type env: dict
        """
        self.namespace = env.get('SERVICE_NAMESPACE', 'default')
        self.log_level = env.get('LOG_LEVEL', 'DEBUG')
        self.config_map_name = env.get('CONFIG_MAP_NAME', 'zedge-services')
        self.output_dir = env.get('OUTPUT_DIR', '/srv/services')


def file_contents_equals_string(file_name, string):
    with open(file_name, 'r') as f:
        return f.read() == string


def handle_config_map_update(object, config, logger):
    """
    :type object: V1ConfigMap
    :type config: SidecarConfig
    :type logger: logging.Logger
    :return:
    """
    name = object.metadata.name
    if name != config.config_map_name:
        return
    new_files = {} if object.data is None else object.data
    old_files = glob.glob(config.output_dir + '/*')
    for file in new_files:
        logger.debug('Considering new file: %s', file)
        path = config.output_dir + '/' + file
        if os.path.exists(path) and file_contents_equals_string(path, new_files[file]):
            # existing file, no changes
            logger.debug('File %s unchanged', file)
            continue
        (fd, tmp_file) = tempfile.mkstemp(dir=config.output_dir)
        with os.fdopen(os.open(fd, os.O_WRONLY | os.O_CREAT, 0o644), 'w') as f:
          f.write(new_files[file])
        os.rename(tmp_file, path)
        logger.info('Updated file %s', path)
    for old_path in old_files:
        old_file = os.path.basename(old_path)
        if old_file not in new_files:
            old_path = config.output_dir + '/' + old_file
            os.unlink(old_path)
            logger.info('Deleted %s', old_path)
            pyc_path = old_path + 'c'
            # also delete *.pyc file if it exists
            if os.path.exists(pyc_path):
                os.unlink(pyc_path)
                logger.info('Deleted %s', pyc_path)


def main(config, logger):
    """
    :type config: SidecarConfig
    :type logger: logging.Logger
    """
    try:
        kubernetes.config.load_incluster_config()
    except kubernetes.config.ConfigException:
        kubernetes.config.load_kube_config()
    v1 = kubernetes.client.CoreV1Api()
    w = kubernetes.watch.Watch()
    logger.info('Watching for changes to ConfigMap {}/{}'.format(config.namespace,
                                                                 config.config_map_name))
    for event in w.stream(v1.list_namespaced_config_map, config.namespace):
        logger.info('Event: %s %s', event['type'], event['object'].metadata.name)
        handle_config_map_update(event['object'], config, logger)


if __name__ == '__main__':
    my_config = SidecarConfig(os.environ)
    my_log_level = logging.getLevelName(my_config.log_level)

    handler = logging.StreamHandler(sys.stdout)
    formatter = StackdriverJsonFormatter()
    handler.setFormatter(formatter)

    my_logger = logging.getLogger(os.path.basename(__file__))
    my_logger.addHandler(handler)
    my_logger.setLevel(my_log_level)

    main(my_config, my_logger)
