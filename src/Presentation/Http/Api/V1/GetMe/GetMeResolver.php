<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\GetMe;

use App\Infrastructure\Persistence\Doctrine\User\User;
use Symfony\Bundle\SecurityBundle\Security;
use Symfony\Component\HttpKernel\Exception\NotFoundHttpException;

readonly class GetMeResolver
{
    public function __construct(
        private Security $security,
    ) {
    }

    public function resolve(): GetMeDto
    {
        /** @var User|null $user */
        $user = $this->security->getUser();

        if (null === $user) {
            throw new NotFoundHttpException('User not found!');
        }

        if (null === $user->getId()) {
            throw new \RuntimeException('User founded with id = 0; WTF?!??!');
        }

        return new GetMeDto(
            $user->getId(),
            $user->getUserIdentifier(),
            $user->getPhone() ?? '',
            $user->getFirstName() ?? '',
            $user->getLastName() ?? '',
            $user->getRoles()
        );
    }
}
