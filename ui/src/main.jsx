import React from "react";
import ReactDOM from "react-dom/client";
import {
  createBrowserRouter,
  RouterProvider,
} from "react-router-dom";
import '@patternfly/react-core/dist/styles/base.css';
import './app.css';

import Root from './routes/root'

const router = createBrowserRouter([
  {
    path: "/",
    element: <Root/>,
  },
]);

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
);
