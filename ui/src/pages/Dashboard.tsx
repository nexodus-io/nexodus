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

import CardImage from "../wordmark.png";

const Dashboard = () => {
  return (
    <div>
      <Card>
        <CardMedia
          component="img"
          height="200"
          image={CardImage}
          alt="nexodus banner"
        />
        <CardHeader title="Welcome to Nexodus" />
        <CardContent>
          Nexodus is a connectivity-as-a-service solution.
        </CardContent>
      </Card>
      <Card>
        <CardHeader title="Download Nexodus Installer" />
        <CardContent>
          Nexodus Installer installs the nexd agent and its dependencies.
        </CardContent>
        <CardActions>
          <Button
            size="small"
            startIcon={<FontAwesomeIcon icon={faDownload as IconProp} />}
            href="https://nexodus-io.s3.amazonaws.com/installer/nexodus-installer.sh"
          >
            Nexodus Installer
          </Button>
        </CardActions>
      </Card>
      <Card>
        <CardHeader title="Download Nexodus Binaries" />
        <CardContent>
          If you want to download only the Nexodus agent to run on your system,
          please download it here.
        </CardContent>
        <CardActions>
          <Button
            size="small"
            startIcon={<FontAwesomeIcon icon={faApple as IconProp} />}
            href="https://nexodus-io.s3.amazonaws.com/nexd-darwin-amd64"
          >
            Download (x86_64)
          </Button>
          <Button
            size="small"
            startIcon={<FontAwesomeIcon icon={faApple as IconProp} />}
            href="https://nexodus-io.s3.amazonaws.com/nexd-darwin-arm64"
          >
            Download (aarch64)
          </Button>
          <Button
            size="small"
            startIcon={<FontAwesomeIcon icon={faWindows as IconProp} />}
            href="https://nexodus-io.s3.amazonaws.com/nexd-windows-amd64"
          >
            Download (x86_64)
          </Button>
          <Button
            size="small"
            startIcon={<FontAwesomeIcon icon={faLinux as IconProp} />}
            href="https://nexodus-io.s3.amazonaws.com/nexd-linux-arm64"
          >
            Download (x86_64)
          </Button>
          <Button
            size="small"
            startIcon={<FontAwesomeIcon icon={faLinux as IconProp} />}
            href="https://nexodus-io.s3.amazonaws.com/nexd-linux-arm64"
          >
            Download (aarch64)
          </Button>
          <Button
            size="small"
            startIcon={<FontAwesomeIcon icon={faLinux as IconProp} />}
            href="https://nexodus-io.s3.amazonaws.com/nexd-linux-arm64"
          >
            Download (arm)
          </Button>
        </CardActions>
      </Card>
      <Card>
        <CardHeader title="QuickStart" />
        <CardContent>
          <Typography variant="body1">
            On a host with the Nexodus agent installed, run the following
            command and follow the instructions it gives you:
          </Typography>
          <Typography variant="body2" color="text.secondary">
            $ sudo nexd {window.location.protocol}//{window.location.host}
          </Typography>
        </CardContent>
      </Card>
    </div>
  );
};

export default Dashboard;
