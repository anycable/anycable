import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';

const client = new grpc.Client();
client.load(['definitions'], '../../../protos/rpc.proto');

let debug = __ENV.DEBUG === '1';

export default () => {
  client.connect('localhost:50051', {timeout: '2s', plaintext: true});

  const command = {
    command: 'message',
    identifier: JSON.stringify({channel: 'BenchmarkChannel'}),
    connection_identifiers: JSON.stringify({uid: __VU}),
    data: JSON.stringify({action: 'echo', message: 'hey-hey-hey'}),
    env: {
      url: 'ws://localhost:8080/cable'
    }
  };
  const response = client.invoke('anycable.RPC/Command', command);

  check(response, {
    'status is OK': (r) => r && r.status === grpc.StatusOK,
  });

  if (debug) {
    console.log(JSON.stringify(response.message));
  }

  client.close();
  sleep(1);
};
