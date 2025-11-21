<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Command;

use App\Application\UseCases\CreateUser\CreateUserResult;
use App\Presentation\Dto\CreateUserDto;
use Symfony\Component\Console\Attribute\Argument;
use Symfony\Component\Console\Attribute\AsCommand;
use Symfony\Component\Console\Command\Command;
use Symfony\Component\Console\Output\OutputInterface;
use Symfony\Component\Messenger\HandleTrait;
use Symfony\Component\Messenger\MessageBusInterface;

#[AsCommand(name: 'app:create-user')]
class CreateUserCommand
{
    use HandleTrait;

    public function __construct(MessageBusInterface $messageBus){
        $this->messageBus = $messageBus;
    }

    public function __invoke(
        #[Argument] string $firstName,
        #[Argument] string $lastName,
        #[Argument] string $email,
        #[Argument] string $password,
        OutputInterface $output
    ): int
    {
        // TODO: валидация аргументов
        $dto = new CreateUserDto($firstName, $lastName, $email, $password);

        /** @var CreateUserResult $result */
        $result = $this->handle(
            new \App\Application\UseCases\CreateUser\CreateUserCommand(
                $dto->firstName,
                $dto->lastName,
                $dto->email,
                $dto->password
            )
        );

        $output->writeln('User successfully generated! id='.$result->id);

        return Command::SUCCESS;
    }
}
