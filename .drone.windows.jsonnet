local pipeline = import 'pipeline.libsonnet';
local name = 'drone-s3-sync';

[
  pipeline.test('windows', 'amd64', '1803'),
  pipeline.build(name, 'windows', 'amd64', '1803'),
  pipeline.build(name, 'windows', 'amd64', '1809'),
  pipeline.notifications('windows', 'amd64', '1809', ['windows-1803', 'windows-1809']),
]
