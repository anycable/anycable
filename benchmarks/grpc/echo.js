import grpc from 'k6/net/grpc';
import { check } from 'k6';

import { Trend } from "k6/metrics";
let connectTrend = new Trend("grpc_connect", true);

const client = new grpc.Client();
client.load(['definitions'], '../../../protos/rpc.proto');

let debug = __ENV.DEBUG === '1';
let callsNum = (__ENV.N | 0) || 100;

export default () => {
  let startConnect = Date.now();
  client.connect('localhost:50051', {timeout: '2s', plaintext: true});
  let endConnect = Date.now();
  connectTrend.add(endConnect - startConnect);

  const command = {
    command: 'message',
    identifier: JSON.stringify({channel: 'BenchmarkChannel'}),
    connection_identifiers: JSON.stringify({uid: __VU}),
    data: JSON.stringify({action: 'echo', message: 'hey-hey-hey'}),
    env: {
      url: 'ws://localhost:8080/cable'
    }
  };

  for (let i = 0; i < callsNum; i++) {
    const response = client.invoke('anycable.RPC/Command', command);

    check(response, {
      'status is OK': (r) => {
        if (r && r.status === grpc.StatusOK) {
          return true
        }

        console.error("Error: ", r.error.message)
        return false
      },
    });

    if (debug) {
      console.log(JSON.stringify(response.message));
    }
  }

  client.close();
};
