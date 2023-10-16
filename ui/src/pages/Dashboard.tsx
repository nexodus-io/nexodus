import {
  Button,
  Card,
  CardActions,
  CardContent,
  CardHeader,
  CardMedia,
  Typography,
} from "@mui/material";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { IconProp } from "@fortawesome/fontawesome-svg-core";
import {
  faApple,
  faWindows,
  faLinux,
} from "@fortawesome/free-brands-svg-icons";
import { faDownload } from "@fortawesome/free-solid-svg-icons";
import { useTheme } from "@mui/material/styles";

import CardImage from "../wordmark.png";
import CardImageDark from "../wordmark_dark.png";
import { backend } from "../common/Api";

const Dashboard = () => {
  const theme = useTheme();
  return (
    <div>
      <Card
        raised
        sx={{
          margin: "0 auto",
          padding: "0.1em",
        }}
      >
        <CardMedia
          component="img"
          height="200"
          image={theme.palette?.mode === "dark" ? CardImageDark : CardImage}
          alt="nexodus banner"
          sx={{ padding: "1em 1em 0 1em", objectFit: "contain" }}
        />
        <CardHeader title="Welcome to Nexodus" />
        <CardContent>
          Nexodus is a connectivity-as-a-service solution.
        </CardContent>
      </Card>
      <Card>
        <CardHeader title="Quick Start" />
        <CardContent>
          <Typography variant="body1">
            See the{" "}
            <a href="https://docs.nexodus.io/quickstart/">quick start</a> guide
            for instructions on how to get started.
          </Typography>
          <Typography variant="body1">
            See the <a href={backend + "/openapi/index.html"}>openapi</a>{" "}
            documentation to view the developer APIs.
          </Typography>
        </CardContent>
      </Card>
    </div>
  );
};

export default Dashboard;
