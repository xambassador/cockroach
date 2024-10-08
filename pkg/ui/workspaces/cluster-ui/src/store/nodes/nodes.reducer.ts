// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

import { cockroach } from "@cockroachlabs/crdb-protobuf-client";
import { createSlice, PayloadAction } from "@reduxjs/toolkit";

import { DOMAIN_NAME, noopReducer } from "../utils";

type INodeStatus = cockroach.server.status.statuspb.INodeStatus;

export type NodesState = {
  data: INodeStatus[];
  lastError: Error;
  valid: boolean;
};

const initialState: NodesState = {
  data: null,
  lastError: null,
  valid: true,
};

const nodesSlice = createSlice({
  name: `${DOMAIN_NAME}/nodes`,
  initialState,
  reducers: {
    received: (state, action: PayloadAction<INodeStatus[]>) => {
      state.data = action.payload;
      state.valid = true;
      state.lastError = null;
    },
    failed: (state, action: PayloadAction<Error>) => {
      state.valid = false;
      state.lastError = action.payload;
    },
    invalidated: state => {
      state.valid = false;
    },
    // Define actions that don't change state
    refresh: noopReducer,
    request: noopReducer,
  },
});

export const { reducer, actions } = nodesSlice;
