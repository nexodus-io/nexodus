import {
    Button,
    Card,
    CardActions,
    CardContent,
    CardHeader,
    CardMedia,
} from '@mui/material';
import {FontAwesomeIcon} from "@fortawesome/react-fontawesome"
import {IconProp} from "@fortawesome/fontawesome-svg-core"
import {faApple, faWindows, faLinux} from "@fortawesome/free-brands-svg-icons"
import {faDownload} from "@fortawesome/free-solid-svg-icons"

import CardImage from "../apex.png"
  
const Dashboard = () => {

    return (
    <div>
        <Card>
            <CardMedia
                component="img"
                height="200"
                image={CardImage}
                alt="apex mountain image"
            />
            <CardHeader title="Welcome to Apex" />
            <CardContent>
                Apex is a connectivity-as-a-service solution.
            </CardContent>
        </Card>
        <Card>
            <CardHeader title="Download Apex Installer" />
            <CardContent>
                Apex Installer installs all the dependencies and setup apex agent.
            </CardContent>
            <CardActions>
                <Button 
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faDownload as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/installer/apex-installer.sh"
                    >Apex Installer
                </Button>
            </CardActions>
        </Card>        
        <Card>
            <CardHeader title="Download Apex Binaries" />
            <CardContent>
                If you want to run client binaries directly on your system, please download the here.
            </CardContent>
            <CardActions>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faApple as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-darwin-amd64"
                >Download (x86_64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faApple as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-darwin-arm64"
                    >Download (aarch64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faWindows as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-windows-amd64"
                >Download (x86_64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faLinux as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-linux-arm64"
                >Download (x86_64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faLinux as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-linux-arm64"
                >Download (aarch64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faLinux as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-linux-arm64"
                >Download (arm)</Button>
            </CardActions>
        </Card>
    </div>
);
};

export default Dashboard;
